// Package m0report executes the backend and conversion evidence matrix.
package m0report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/selected"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

// Options controls one evidence run.
type Options struct {
	FixtureRoot string
	Runs        int
}

// Evidence contains scalar case results and deduplicated backend shapes.
type Evidence struct {
	Report testkit.Report
	Shapes ShapeReport
}

// ShapeReport stores one reviewable snapshot per unique fixture source.
type ShapeReport struct {
	SchemaVersion int             `json:"schemaVersion"`
	Shapes        []ShapeSnapshot `json:"shapes"`
}

// ShapeSnapshot is a canonical backend-neutral tree and its digest.
type ShapeSnapshot struct {
	Path     string         `json:"path"`
	SHA256   string         `json:"sha256"`
	Snapshot backend.Result `json:"snapshot"`
}

// Run executes manifest serially in manifest order.
func Run(
	ctx context.Context,
	manifest testkit.Manifest,
	options Options,
) (Evidence, error) {
	if options.FixtureRoot == "" {
		return Evidence{}, fmt.Errorf("run M0 report: fixture root is empty")
	}
	if options.Runs < 1 {
		return Evidence{}, fmt.Errorf("run M0 report: runs = %d, want at least 1", options.Runs)
	}
	report := testkit.Report{
		SchemaVersion: 1,
		Run: testkit.RunMetadata{
			GoVersion:      runtime.Version(),
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			CSTGoCommit:    java25.SharedCSTCommit,
			RuntimeVersion: java25.RuntimeVersion,
			RuntimeCommit:  java25.RuntimeCommit,
			GrammarCommit:  java25.GrammarBaseCommit,
		},
		Cases: make([]testkit.CaseResult, 0, len(manifest.Fixtures)),
	}
	shapes := ShapeReport{
		SchemaVersion: 1,
		Shapes:        make([]ShapeSnapshot, 0),
	}
	shapeDigests := make(map[string]string)

	for _, fixture := range manifest.Fixtures {
		if err := ctx.Err(); err != nil {
			return Evidence{}, fmt.Errorf("run M0 report: %w", err)
		}
		raw, err := os.ReadFile(filepath.Join(options.FixtureRoot, filepath.FromSlash(fixture.Path)))
		if err != nil {
			return Evidence{}, fmt.Errorf("read fixture %q: %w", fixture.ID, err)
		}
		result, snapshot, err := measureCase(
			ctx,
			fixture,
			string(raw),
			options.Runs,
			selected.ParseTranslation,
		)
		if err != nil {
			return Evidence{}, fmt.Errorf("measure fixture %q: %w", fixture.ID, err)
		}
		canonical, digest, err := canonicalShape(snapshot)
		if err != nil {
			return Evidence{}, fmt.Errorf("snapshot fixture %q: %w", fixture.ID, err)
		}
		result.BackendShapeSHA256 = digest
		report.Cases = append(report.Cases, result)
		previousDigest, exists := shapeDigests[fixture.Path]
		if exists && previousDigest != digest {
			return Evidence{}, fmt.Errorf(
				"fixture source %q produced level-dependent backend shapes: %s and %s",
				fixture.Path,
				previousDigest,
				digest,
			)
		}
		if !exists {
			shapeDigests[fixture.Path] = digest
			shapes.Shapes = append(shapes.Shapes, ShapeSnapshot{
				Path:     fixture.Path,
				SHA256:   digest,
				Snapshot: canonical,
			})
		}
	}

	return Evidence{Report: report, Shapes: shapes}, nil
}

type pipelineResult struct {
	snapshot    backend.Result
	conversion  convert.Result
	diagnostics int
}

type parseTranslationFunc func(
	context.Context,
	*source.Translation,
	language.Level,
) (backend.Result, error)

func measureCase(
	ctx context.Context,
	fixture testkit.Fixture,
	raw string,
	runs int,
	parse parseTranslationFunc,
) (result testkit.CaseResult, snapshot backend.Result, err error) {
	result = testkit.CaseResult{
		ID:      fixture.ID,
		Release: uint8(fixture.Release),
		Preview: fixture.Preview,
		Feature: fixture.Feature,
		Bytes:   len(raw),
		Notes:   []string{},
	}
	defer func() {
		if value := recover(); value != nil {
			result.Panic = true
			result.Classification = "runtime defect"
			result.Notes = append(result.Notes, fmt.Sprintf("panic: %v", value))
			err = nil
		}
	}()

	level := language.Level{Release: fixture.Release, Preview: fixture.Preview}
	if fixture.Feature != "" {
		feature, parseErr := language.ParseFeatureID(fixture.Feature)
		if parseErr != nil {
			return result, backend.Result{}, parseErr
		}
		support := level.Feature(feature)
		result.FeatureState = support.State.String()
		result.FeatureVariant = support.Variant
		result.FeatureEnabled = level.Supports(feature)
	}

	if _, err := runPipeline(ctx, raw, level, parse); err != nil {
		return result, backend.Result{}, err
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	var pipeline pipelineResult
	for range runs {
		pipeline, err = runPipeline(ctx, raw, level, parse)
		if err != nil {
			return result, backend.Result{}, err
		}
	}
	elapsed := time.Since(start)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	result.ElapsedNS = elapsed.Nanoseconds() / int64(runs)
	result.AllocatedBytes = (after.TotalAlloc - before.TotalAlloc) / uint64(runs)
	result.BackendNodes = pipeline.snapshot.NodeCount
	result.BackendErrors = pipeline.snapshot.ErrorCount
	result.MissingNodes = pipeline.snapshot.MissingCount
	result.ConvertedElements = int(pipeline.conversion.ConvertedElements)
	result.RoundTrip = pipeline.conversion.Tree.Root().AppendText() == raw
	result.SpanInvariants, result.Notes = validateTree(
		pipeline.conversion.Tree,
		raw,
		result.Notes,
	)
	if pipeline.diagnostics != 0 {
		result.Notes = append(
			result.Notes,
			fmt.Sprintf("Unicode translation diagnostics: %d", pipeline.diagnostics),
		)
	}
	classify(&result)

	return result, pipeline.snapshot, nil
}

func canonicalShape(snapshot backend.Result) (backend.Result, string, error) {
	snapshot.Level = language.Level{}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return backend.Result{}, "", fmt.Errorf("encode canonical backend shape: %w", err)
	}
	digest := sha256.Sum256(data)

	return snapshot, hex.EncodeToString(digest[:]), nil
}

func runPipeline(
	ctx context.Context,
	raw string,
	level language.Level,
	parse parseTranslationFunc,
) (pipelineResult, error) {
	translation := source.Translate(raw)
	snapshot, err := parse(ctx, translation, level)
	if err != nil {
		return pipelineResult{}, err
	}
	conversion, err := convert.ConvertTranslation(translation, snapshot)
	if err != nil {
		return pipelineResult{}, err
	}

	return pipelineResult{
		snapshot:    snapshot,
		conversion:  conversion,
		diagnostics: len(translation.Diagnostics()),
	}, nil
}

func validateTree(
	tree *syntax.CoreTree,
	raw string,
	notes []string,
) (bool, []string) {
	if tree == nil {
		return false, append(notes, "converted tree is nil")
	}
	root := tree.Root()
	valid := true
	if root.Green().FullWidth() != len(raw) {
		valid = false
		notes = append(notes, "root full width differs from input length")
	}
	if root.FullSpan().Start != 0 || root.FullSpan().End != len(raw) {
		valid = false
		notes = append(notes, "root full span differs from input range")
	}

	seen := map[syntax.ElementID]struct{}{root.ID(): {}}
	for node := range root.DescendantNodes() {
		if _, exists := seen[node.ID()]; exists {
			valid = false
			notes = append(notes, "duplicate node occurrence ID")
			break
		}
		seen[node.ID()] = struct{}{}
		if !contains(root.FullSpan(), node.FullSpan()) {
			valid = false
			notes = append(notes, "node span outside root")
			break
		}
	}
	for token := range root.DescendantTokens() {
		if _, exists := seen[token.ID()]; exists {
			valid = false
			notes = append(notes, "duplicate token occurrence ID")
			break
		}
		seen[token.ID()] = struct{}{}
		if !contains(root.FullSpan(), token.FullSpan()) {
			valid = false
			notes = append(notes, "token span outside root")
			break
		}
	}

	return valid, notes
}

func contains(outer, inner syntax.Span) bool {
	return outer.Start <= inner.Start && inner.End <= outer.End
}

func classify(result *testkit.CaseResult) {
	switch {
	case !result.RoundTrip || !result.SpanInvariants:
		result.Classification = "repository conversion defect"
	case result.Feature != "" && !result.FeatureEnabled:
		result.Classification = "post-parse feature-validation requirement"
		if result.BackendErrors == 0 {
			result.Notes = append(
				result.Notes,
				"release-agnostic backend accepted syntax disabled at this level",
			)
		}
	case result.BackendErrors != 0:
		result.Classification = "upstream Java grammar gap"
		result.Notes = append(
			result.Notes,
			fmt.Sprintf("backend produced %d error nodes", result.BackendErrors),
		)
	}
}
