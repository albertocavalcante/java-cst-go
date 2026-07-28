package oracle_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestRunCompilerOracleMatchesExpectedOutcomes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "accept.java"), []byte("class A {}"), 0o600); err != nil {
		t.Fatalf("write accepted fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "reject.java"), []byte("broken"), 0o600); err != nil {
		t.Fatalf("write rejected fixture: %v", err)
	}
	installation := fakeCompiler(t, `#!/bin/sh
case "$*" in
  *reject.java*)
    echo "reject.java:1:1: compiler.err.expected: class, interface, enum, or record" >&2
    exit 1
    ;;
esac
exit 0
`)
	toolchain := oracle.Toolchain{
		Release: language.Release26,
		Version: "26-test",
		Build:   "26-test+1",
		OS:      "test",
		Arch:    "test",
		SHA256:  strings.Repeat("a", 64),
	}
	manifest := testkit.Manifest{
		SchemaVersion: 2,
		Fixtures: []testkit.Fixture{
			{
				ID:              "accept",
				Path:            "accept.java",
				Release:         language.Release26,
				Category:        "release-anchor",
				ExpectedBackend: "measure",
			},
			{
				ID:                   "reject",
				Path:                 "reject.java",
				Release:              language.Release26,
				Preview:              true,
				Category:             "feature-boundary",
				Feature:              "string-templates",
				ExpectedFeatureState: "withdrawn",
				ExpectedBackend:      "measure",
			},
		},
	}

	report, err := oracle.Run(
		context.Background(),
		installation,
		toolchain,
		manifest,
		root,
		time.Second,
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := len(report.Cases), 2; got != want {
		t.Fatalf("case count = %d, want %d", got, want)
	}
	for _, result := range report.Cases {
		if !result.Matches {
			t.Errorf("case %q does not match: %+v", result.ID, result)
		}
	}
	if report.Cases[1].Accepted || report.Cases[1].ExitCode != 1 {
		t.Fatalf("rejected case = %+v", report.Cases[1])
	}
	if got := report.Summary(); got != "2/2 matched" {
		t.Fatalf("Summary() = %q, want %q", got, "2/2 matched")
	}
}

func TestCompileHonorsContextDeadline(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "slow.java"), nil, 0o600); err != nil {
		t.Fatalf("write slow fixture: %v", err)
	}
	installation := fakeCompiler(t, "#!/bin/sh\nexec sleep 10\n")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := oracle.Compile(ctx, installation, root, testkit.Fixture{
		Path:    "slow.java",
		Release: language.Release26,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Compile error = %v, want context deadline exceeded", err)
	}
}

func TestCorrelateBackendConfirmsGrammarGap(t *testing.T) {
	t.Parallel()

	report := oracle.Report{Cases: []oracle.CaseResult{{
		ID:               "module-import",
		ExpectedAccepted: true,
		Accepted:         true,
		Matches:          true,
	}}}
	backend := testkit.Report{Cases: []testkit.CaseResult{{
		ID:             "module-import",
		BackendErrors:  1,
		RoundTrip:      true,
		SpanInvariants: true,
	}}}
	if err := oracle.CorrelateBackend(&report, backend); err != nil {
		t.Fatalf("CorrelateBackend: %v", err)
	}
	result := report.Cases[0]
	if !result.BackendMeasured || result.BackendErrors != 1 {
		t.Fatalf("backend correlation = %+v", result)
	}
	if got, want := result.DifferentialClassification, "confirmed upstream Java grammar gap"; got != want {
		t.Fatalf("classification = %q, want %q", got, want)
	}
}

func fakeCompiler(t *testing.T, script string) oracle.Installation {
	t.Helper()

	javaHome := filepath.Join(t.TempDir(), "jdk", "Contents", "Home")
	javac := filepath.Join(javaHome, "bin", "javac")
	if err := os.MkdirAll(filepath.Dir(javac), 0o755); err != nil {
		t.Fatalf("create fake JDK: %v", err)
	}
	if err := os.WriteFile(javac, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake javac: %v", err)
	}

	return oracle.Installation{
		Root:     filepath.Dir(filepath.Dir(javaHome)),
		JavaHome: javaHome,
		Javac:    javac,
	}
}
