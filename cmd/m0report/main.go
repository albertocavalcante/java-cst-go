// Command m0report writes machine-readable M0 backend evidence.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"git.alberto.engineer/alberto/java-cst-go/internal/m0report"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	manifestPath := flag.String(
		"manifest",
		"testdata/m0/fixtures.json",
		"path to the M0 fixture manifest",
	)
	fixtureRoot := flag.String(
		"fixture-root",
		"testdata/m0",
		"directory containing paths from the fixture manifest",
	)
	outputPath := flag.String(
		"out",
		"reports/m0/results.json",
		"path for the generated JSON report",
	)
	shapesPath := flag.String(
		"shapes-out",
		"reports/m0/backend-shapes.json",
		"path for deduplicated backend shape snapshots",
	)
	runs := flag.Int("runs", 5, "measured pipeline repetitions per fixture")
	backend := flag.String(
		"backend",
		string(m0report.BackendPinned),
		"backend to measure: pinned or java25",
	)
	flag.Parse()

	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := testkit.DecodeManifest(bytes.NewReader(manifestData))
	if err != nil {
		return err
	}
	evidence, err := m0report.Run(context.Background(), manifest, m0report.Options{
		FixtureRoot: *fixtureRoot,
		Runs:        *runs,
		Backend:     m0report.Backend(*backend),
	})
	if err != nil {
		return err
	}

	if err := writeJSON(*shapesPath, evidence.Shapes); err != nil {
		return err
	}

	return writeJSON(*outputPath, evidence.Report)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".m0report-*.json")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return errors.Join(
			fmt.Errorf("encode report: %w", err),
			file.Close(),
		)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close report: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish report: %w", err)
	}

	return nil
}
