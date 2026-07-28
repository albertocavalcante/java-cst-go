package oracle_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestCheckedJavacEvidenceCoversJava21Through26(t *testing.T) {
	t.Parallel()

	lock, err := oracle.LoadToolchainLock()
	if err != nil {
		t.Fatalf("LoadToolchainLock: %v", err)
	}
	manifestData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "m0", "fixtures.json"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	manifest, err := testkit.DecodeManifest(bytes.NewReader(manifestData))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}

	expected := make(map[string]struct{})
	for _, fixture := range manifest.Fixtures {
		if fixture.Release >= language.Release21 {
			expected[fixture.ID] = struct{}{}
		}
	}

	observed := make(map[string]struct{})
	for release := language.Release21; release <= language.Release26; release++ {
		path := filepath.Join(
			"..",
			"..",
			"reports",
			"m0",
			fmt.Sprintf("javac-results-java%s.json", release),
		)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Java %s evidence: %v", release, err)
		}
		var report oracle.Report
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatalf("decode Java %s evidence: %v", release, err)
		}
		toolchain, err := lock.Select(release, "darwin", "arm64")
		if err != nil {
			t.Fatalf("select Java %s toolchain: %v", release, err)
		}
		if report.Compiler.Release != release ||
			report.Compiler.Version != toolchain.Version ||
			report.Compiler.Build != toolchain.Build ||
			report.Compiler.SHA256 != toolchain.SHA256 {
			t.Errorf(
				"Java %s evidence compiler = %+v, want locked %+v",
				release,
				report.Compiler,
				toolchain,
			)
		}
		for _, result := range report.Cases {
			if result.Release != uint8(release) {
				t.Errorf("case %q release = %d, want %s", result.ID, result.Release, release)
			}
			if !result.Matches {
				t.Errorf("case %q compiler result does not match expectation", result.ID)
			}
			if !result.BackendMeasured {
				t.Errorf("case %q has no correlated backend evidence", result.ID)
			}
			if _, duplicate := observed[result.ID]; duplicate {
				t.Errorf("duplicate compiler evidence for %q", result.ID)
			}
			observed[result.ID] = struct{}{}
		}
	}

	for id := range expected {
		if _, exists := observed[id]; !exists {
			t.Errorf("missing compiler evidence for %q", id)
		}
	}
	for id := range observed {
		if _, exists := expected[id]; !exists {
			t.Errorf("compiler evidence for unknown or out-of-scope fixture %q", id)
		}
	}
}
