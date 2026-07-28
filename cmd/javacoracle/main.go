// Command javacoracle runs the locked compiler against matching M0 fixtures.
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
	"runtime"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	releaseNumber := flag.Int("release", 26, "matching Java fixture/compiler release")
	cache := flag.String("cache", "", "oracle cache directory")
	manifestPath := flag.String(
		"manifest",
		"testdata/m0/fixtures.json",
		"path to the M0 fixture manifest",
	)
	fixtureRoot := flag.String(
		"fixture-root",
		"testdata/m0",
		"directory containing fixture paths",
	)
	outputPath := flag.String(
		"out",
		"reports/m0/javac-results.json",
		"path for compiler evidence",
	)
	backendResultsPath := flag.String(
		"backend-results",
		"reports/m0/results.json",
		"path to backend/conversion evidence for correlation",
	)
	timeout := flag.Duration("timeout", 10*time.Second, "per-fixture javac timeout")
	flag.Parse()

	release := language.Release(*releaseNumber)
	if !release.Valid() {
		return fmt.Errorf("invalid Java release %d: want 8 through 26", *releaseNumber)
	}
	lock, err := oracle.LoadToolchainLock()
	if err != nil {
		return err
	}
	toolchain, err := lock.Select(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if *cache == "" {
		*cache, err = oracle.DefaultCacheDir()
		if err != nil {
			return err
		}
	}
	installation, err := (oracle.Installer{CacheDir: *cache}).Ensure(
		context.Background(),
		toolchain,
	)
	if err != nil {
		return err
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		return fmt.Errorf("read fixture manifest: %w", err)
	}
	manifest, err := testkit.DecodeManifest(bytes.NewReader(manifestData))
	if err != nil {
		return err
	}
	report, err := oracle.Run(
		context.Background(),
		installation,
		toolchain,
		manifest,
		*fixtureRoot,
		*timeout,
	)
	if err != nil {
		return err
	}
	backendData, err := os.ReadFile(*backendResultsPath)
	if err != nil {
		return fmt.Errorf("read backend results: %w", err)
	}
	var backendReport testkit.Report
	if err := json.Unmarshal(backendData, &backendReport); err != nil {
		return fmt.Errorf("decode backend results: %w", err)
	}
	if err := oracle.CorrelateBackend(&report, backendReport); err != nil {
		return err
	}
	if err := writeJSON(*outputPath, report); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stderr, report.Summary()); err != nil {
		return fmt.Errorf("write javac summary: %w", err)
	}

	return nil
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".javacoracle-*.json")
	if err != nil {
		return fmt.Errorf("create temporary javac report: %w", err)
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
			fmt.Errorf("encode javac report: %w", err),
			file.Close(),
		)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close javac report: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("publish javac report: %w", err)
	}

	return nil
}
