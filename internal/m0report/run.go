// Package m0report executes the backend and conversion evidence matrix.
package m0report

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/internal/grammar"
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

// Run executes manifest serially in manifest order.
func Run(
	ctx context.Context,
	manifest testkit.Manifest,
	options Options,
) (testkit.Report, error) {
	if options.FixtureRoot == "" {
		return testkit.Report{}, fmt.Errorf("run M0 report: fixture root is empty")
	}
	if options.Runs < 1 {
		return testkit.Report{}, fmt.Errorf("run M0 report: runs = %d, want at least 1", options.Runs)
	}

	lock, err := grammar.Load()
	if err != nil {
		return testkit.Report{}, fmt.Errorf("run M0 report: %w", err)
	}
	report := testkit.Report{
		SchemaVersion: 1,
		Run: testkit.RunMetadata{
			GoVersion:      runtime.Version(),
			OS:             runtime.GOOS,
			Arch:           runtime.GOARCH,
			CSTGoCommit:    lock.SharedCST.Commit,
			RuntimeVersion: lock.Runtime.Version,
			RuntimeCommit:  lock.Runtime.Commit,
			GrammarCommit:  lock.Grammar.Commit,
		},
		Cases: make([]testkit.CaseResult, 0, len(manifest.Fixtures)),
	}

	for _, fixture := range manifest.Fixtures {
		if err := ctx.Err(); err != nil {
			return testkit.Report{}, fmt.Errorf("run M0 report: %w", err)
		}
		raw, err := os.ReadFile(filepath.Join(options.FixtureRoot, filepath.FromSlash(fixture.Path)))
		if err != nil {
			return testkit.Report{}, fmt.Errorf("read fixture %q: %w", fixture.ID, err)
		}
		result, err := measureCase(ctx, fixture, string(raw), options.Runs)
		if err != nil {
			return testkit.Report{}, fmt.Errorf("measure fixture %q: %w", fixture.ID, err)
		}
		report.Cases = append(report.Cases, result)
	}

	return report, nil
}

type pipelineResult struct {
	snapshot    backend.Result
	conversion  convert.Result
	diagnostics int
}

func measureCase(
	ctx context.Context,
	fixture testkit.Fixture,
	raw string,
	runs int,
) (result testkit.CaseResult, err error) {
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
			return result, parseErr
		}
		support := level.Feature(feature)
		result.FeatureState = support.State.String()
		result.FeatureVariant = support.Variant
		result.FeatureEnabled = level.Supports(feature)
	}

	if _, err := runPipeline(ctx, raw, level); err != nil {
		return result, err
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	var pipeline pipelineResult
	for range runs {
		pipeline, err = runPipeline(ctx, raw, level)
		if err != nil {
			return result, err
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

	return result, nil
}

func runPipeline(
	ctx context.Context,
	raw string,
	level language.Level,
) (pipelineResult, error) {
	translation := source.Translate(raw)
	snapshot, err := treesitter.ParseTranslation(ctx, translation, level)
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
	tree *syntax.Tree,
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
