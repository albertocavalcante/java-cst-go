package oracle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

// CompileResult records one deterministic javac invocation.
type CompileResult struct {
	Accepted    bool
	ExitCode    int
	Stdout      string
	Diagnostics []string
	ElapsedNS   int64
}

// Compile invokes the installed compiler for one fixture.
func Compile(
	ctx context.Context,
	installation Installation,
	fixtureRoot string,
	fixture testkit.Fixture,
) (CompileResult, error) {
	if fixtureRoot == "" {
		return CompileResult{}, errors.New("run javac oracle: fixture root is empty")
	}
	if err := verifyInstallation(installation); err != nil {
		return CompileResult{}, err
	}

	output, err := os.MkdirTemp("", "java-cst-go-javac-*")
	if err != nil {
		return CompileResult{}, fmt.Errorf("create javac output directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(output)
	}()

	arguments := []string{
		"-XDrawDiagnostics",
		"-proc:none",
		"-encoding", "UTF-8",
		"-d", output,
		"--release", fixture.Release.String(),
	}
	if fixture.Preview {
		arguments = append(arguments, "--enable-preview")
	}
	arguments = append(arguments, filepath.FromSlash(fixture.Path))

	command := exec.CommandContext(ctx, installation.Javac, arguments...)
	command.Dir = fixtureRoot
	command.Env = oracleEnvironment(installation, output)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	start := time.Now()
	runErr := command.Run()
	elapsed := time.Since(start)
	result := CompileResult{
		Accepted:    runErr == nil,
		ExitCode:    0,
		Stdout:      strings.TrimSpace(stdout.String()),
		Diagnostics: diagnosticLines(stderr.String()),
		ElapsedNS:   elapsed.Nanoseconds(),
	}
	if runErr == nil {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return CompileResult{}, fmt.Errorf("run javac oracle: %w", ctxErr)
	}
	var exitError *exec.ExitError
	if !errors.As(runErr, &exitError) {
		return CompileResult{}, fmt.Errorf("run javac oracle: %w", runErr)
	}
	result.ExitCode = exitError.ExitCode()

	return result, nil
}

func oracleEnvironment(installation Installation, temporaryDirectory string) []string {
	return []string{
		"JAVA_HOME=" + installation.JavaHome,
		"LANG=C",
		"LC_ALL=C",
		"PATH=" + filepath.Join(installation.JavaHome, "bin") + ":/usr/bin:/bin",
		"TMPDIR=" + temporaryDirectory,
		"TZ=UTC",
	}
}

func diagnosticLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{}
	}

	return strings.Split(text, "\n")
}

// Report is one compiler-backed feature-boundary evidence run.
type Report struct {
	SchemaVersion int          `json:"schemaVersion"`
	Compiler      CompilerInfo `json:"compiler"`
	Cases         []CaseResult `json:"cases"`
}

// CompilerInfo identifies the exact locked javac used by a report.
type CompilerInfo struct {
	Release language.Release `json:"release"`
	Version string           `json:"version"`
	Build   string           `json:"build"`
	OS      string           `json:"os"`
	Arch    string           `json:"arch"`
	SHA256  string           `json:"sha256"`
}

// CaseResult compares javac behavior with the fixture's expected feature state.
type CaseResult struct {
	ID                         string   `json:"id"`
	Release                    uint8    `json:"release"`
	Preview                    bool     `json:"preview"`
	Feature                    string   `json:"feature,omitempty"`
	ExpectedAccepted           bool     `json:"expectedAccepted"`
	Accepted                   bool     `json:"accepted"`
	ExitCode                   int      `json:"exitCode"`
	Diagnostics                []string `json:"diagnostics"`
	ElapsedNS                  int64    `json:"elapsedNs"`
	Matches                    bool     `json:"matches"`
	BackendMeasured            bool     `json:"backendMeasured"`
	BackendErrors              int      `json:"backendErrors,omitempty"`
	BackendClassification      string   `json:"backendClassification,omitempty"`
	DifferentialClassification string   `json:"differentialClassification,omitempty"`
}

// Run executes fixtures for exactly the compiler's matching release.
func Run(
	ctx context.Context,
	installation Installation,
	toolchain Toolchain,
	manifest testkit.Manifest,
	fixtureRoot string,
	timeout time.Duration,
) (Report, error) {
	if timeout <= 0 {
		return Report{}, fmt.Errorf("run javac oracle: timeout = %s, want positive", timeout)
	}
	report := Report{
		SchemaVersion: 1,
		Compiler: CompilerInfo{
			Release: toolchain.Release,
			Version: toolchain.Version,
			Build:   toolchain.Build,
			OS:      toolchain.OS,
			Arch:    toolchain.Arch,
			SHA256:  toolchain.SHA256,
		},
		Cases: make([]CaseResult, 0),
	}

	for _, fixture := range manifest.Fixtures {
		if fixture.Release != toolchain.Release {
			continue
		}
		caseContext, cancel := context.WithTimeout(ctx, timeout)
		result, err := Compile(caseContext, installation, fixtureRoot, fixture)
		cancel()
		if err != nil {
			return Report{}, fmt.Errorf("compile fixture %q: %w", fixture.ID, err)
		}
		expected, err := fixture.ExpectsJavacAcceptance()
		if err != nil {
			return Report{}, fmt.Errorf("fixture %q: %w", fixture.ID, err)
		}
		report.Cases = append(report.Cases, CaseResult{
			ID:               fixture.ID,
			Release:          uint8(fixture.Release),
			Preview:          fixture.Preview,
			Feature:          fixture.Feature,
			ExpectedAccepted: expected,
			Accepted:         result.Accepted,
			ExitCode:         result.ExitCode,
			Diagnostics:      result.Diagnostics,
			ElapsedNS:        result.ElapsedNS,
			Matches:          result.Accepted == expected,
		})
	}
	if len(report.Cases) == 0 {
		return Report{}, fmt.Errorf(
			"run javac oracle: manifest has no Java %s fixtures",
			toolchain.Release,
		)
	}

	return report, nil
}

// CorrelateBackend joins compiler results with the checked-in backend report.
func CorrelateBackend(report *Report, backendReport testkit.Report) error {
	if report == nil {
		return errors.New("correlate javac oracle: nil report")
	}
	backendCases := make(map[string]testkit.CaseResult, len(backendReport.Cases))
	for _, result := range backendReport.Cases {
		backendCases[result.ID] = result
	}
	for index := range report.Cases {
		result := &report.Cases[index]
		backendResult, exists := backendCases[result.ID]
		if !exists {
			return fmt.Errorf(
				"correlate javac oracle: backend report has no case %q",
				result.ID,
			)
		}
		result.BackendMeasured = true
		result.BackendErrors = backendResult.BackendErrors
		result.BackendClassification = backendResult.Classification
		switch {
		case !result.Matches:
			result.DifferentialClassification = "fixture-or-oracle mismatch"
		case !backendResult.RoundTrip || !backendResult.SpanInvariants:
			result.DifferentialClassification = "repository conversion defect"
		case result.ExpectedAccepted && result.Accepted && backendResult.BackendErrors > 0:
			result.DifferentialClassification = "confirmed upstream Java grammar gap"
		case !result.ExpectedAccepted:
			result.DifferentialClassification = "post-parse feature-validation requirement"
		default:
			result.DifferentialClassification = "aligned"
		}
	}

	return nil
}

// Summary returns a compact human-readable report status.
func (report Report) Summary() string {
	matches := 0
	for _, result := range report.Cases {
		if result.Matches {
			matches++
		}
	}

	return strconv.Itoa(matches) + "/" + strconv.Itoa(len(report.Cases)) + " matched"
}
