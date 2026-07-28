package java25_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
)

type generatedLock struct {
	SchemaVersion int `json:"schemaVersion"`
	SharedCST     struct {
		Module  string `json:"module"`
		Version string `json:"version"`
		Commit  string `json:"commit"`
	} `json:"sharedCST"`
	Runtime struct {
		Module                    string `json:"module"`
		Version                   string `json:"version"`
		Commit                    string `json:"commit"`
		ModuleSum                 string `json:"moduleSum"`
		UpstreamTreeSitterVersion string `json:"upstreamTreeSitterVersion"`
	} `json:"runtime"`
	ParserGenerator struct {
		Version       string `json:"version"`
		ArchiveURL    string `json:"archiveURL"`
		ArchiveSHA256 string `json:"archiveSHA256"`
	} `json:"parserGenerator"`
	TableGenerator struct {
		Module      string `json:"module"`
		Version     string `json:"version"`
		Commit      string `json:"commit"`
		Command     string `json:"command"`
		Postprocess string `json:"postprocess"`
	} `json:"tableGenerator"`
	Grammar struct {
		Repository             string `json:"repository"`
		BaseCommit             string `json:"baseCommit"`
		PatchPath              string `json:"patchPath"`
		PatchSHA256            string `json:"patchSHA256"`
		PatchedGrammarSHA256   string `json:"patchedGrammarSHA256"`
		GeneratedParserCBytes  int    `json:"generatedParserCBytes"`
		GeneratedParserCSHA256 string `json:"generatedParserCSHA256"`
		GeneratedGoPath        string `json:"generatedGoPath"`
		GeneratedGoBytes       int    `json:"generatedGoBytes"`
		GeneratedGoSHA256      string `json:"generatedGoSHA256"`
	} `json:"grammar"`
}

func TestGeneratedTableMatchesLock(t *testing.T) {
	t.Parallel()

	lock := loadLock(t)
	if lock.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", lock.SchemaVersion)
	}
	for name, values := range map[string][2]string{
		"shared CST version": {lock.SharedCST.Version, java25.SharedCSTVersion},
		"shared CST commit":  {lock.SharedCST.Commit, java25.SharedCSTCommit},
		"runtime version":    {lock.Runtime.Version, java25.RuntimeVersion},
		"runtime commit":     {lock.Runtime.Commit, java25.RuntimeCommit},
		"grammar commit":     {lock.Grammar.BaseCommit, java25.GrammarBaseCommit},
	} {
		if values[0] != values[1] {
			t.Errorf("%s lock = %q, constant = %q", name, values[0], values[1])
		}
	}
	for name, value := range map[string]string{
		"shared CST module":       lock.SharedCST.Module,
		"shared CST version":      lock.SharedCST.Version,
		"runtime module":          lock.Runtime.Module,
		"runtime version":         lock.Runtime.Version,
		"runtime module sum":      lock.Runtime.ModuleSum,
		"runtime tree-sitter":     lock.Runtime.UpstreamTreeSitterVersion,
		"parser generator":        lock.ParserGenerator.Version,
		"parser generator URL":    lock.ParserGenerator.ArchiveURL,
		"table generator module":  lock.TableGenerator.Module,
		"table generator version": lock.TableGenerator.Version,
		"table generator command": lock.TableGenerator.Command,
		"table postprocess":       lock.TableGenerator.Postprocess,
		"grammar repository":      lock.Grammar.Repository,
		"grammar patch path":      lock.Grammar.PatchPath,
		"generated Go path":       lock.Grammar.GeneratedGoPath,
	} {
		if strings.TrimSpace(value) == "" {
			t.Errorf("%s is empty", name)
		}
	}
	for name, value := range map[string]string{
		"parser archive SHA-256":   lock.ParserGenerator.ArchiveSHA256,
		"patch SHA-256":            lock.Grammar.PatchSHA256,
		"patched grammar SHA-256":  lock.Grammar.PatchedGrammarSHA256,
		"generated parser SHA-256": lock.Grammar.GeneratedParserCSHA256,
		"generated Go SHA-256":     lock.Grammar.GeneratedGoSHA256,
	} {
		assertHexDigest(t, name, value)
	}
	for name, value := range map[string]string{
		"shared CST commit":      lock.SharedCST.Commit,
		"runtime commit":         lock.Runtime.Commit,
		"table generator commit": lock.TableGenerator.Commit,
		"grammar base commit":    lock.Grammar.BaseCommit,
	} {
		assertCommit(t, name, value)
	}

	assertFile(t, lock.Grammar.PatchPath, lock.Grammar.PatchSHA256, 0)
	assertFile(
		t,
		lock.Grammar.GeneratedGoPath,
		lock.Grammar.GeneratedGoSHA256,
		lock.Grammar.GeneratedGoBytes,
	)
	if lock.Grammar.GeneratedParserCBytes <= 0 {
		t.Errorf(
			"generated parser.c bytes = %d, want positive",
			lock.Grammar.GeneratedParserCBytes,
		)
	}
}

func loadLock(t *testing.T) generatedLock {
	t.Helper()

	file, err := os.Open("generated.lock.json")
	if err != nil {
		t.Fatalf("open generated lock: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close generated lock: %v", err)
		}
	}()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var lock generatedLock
	if err := decoder.Decode(&lock); err != nil {
		t.Fatalf("decode generated lock: %v", err)
	}
	return lock
}

func assertHexDigest(t *testing.T, name, value string) {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Errorf("%s: decode: %v", name, err)
		return
	}
	if len(decoded) != sha256.Size {
		t.Errorf("%s: digest is %d bytes, want %d", name, len(decoded), sha256.Size)
	}
}

func assertCommit(t *testing.T, name, value string) {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil {
		t.Errorf("%s: decode: %v", name, err)
		return
	}
	const gitCommitBytes = 20
	if len(decoded) != gitCommitBytes {
		t.Errorf("%s: commit is %d bytes, want %d", name, len(decoded), gitCommitBytes)
	}
}

func assertFile(t *testing.T, path, wantSHA string, wantBytes int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if wantBytes > 0 && len(data) != wantBytes {
		t.Errorf("%s bytes = %d, want %d", path, len(data), wantBytes)
	}
	gotSHA := fmt.Sprintf("%x", sha256.Sum256(data))
	if gotSHA != wantSHA {
		t.Errorf("%s SHA-256 = %s, want %s", path, gotSHA, wantSHA)
	}
}
