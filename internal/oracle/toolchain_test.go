package oracle_test

import (
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestLockedOracleToolchainsCoverJava21Through26(t *testing.T) {
	t.Parallel()

	lock, err := oracle.LoadToolchainLock()
	if err != nil {
		t.Fatalf("LoadToolchainLock: %v", err)
	}
	if got, want := len(lock.Toolchains), 6; got != want {
		t.Fatalf("toolchain count = %d, want %d", got, want)
	}
	for release := language.Release21; release <= language.Release26; release++ {
		toolchain, err := lock.Select(release, "darwin", "arm64")
		if err != nil {
			t.Errorf("Select(Java %s): %v", release, err)
			continue
		}
		if toolchain.Release != release {
			t.Errorf("Select(Java %s).Release = %s", release, toolchain.Release)
		}
		if !strings.HasPrefix(toolchain.ArchiveURL, "https://download.java.net/") {
			t.Errorf("Java %s archive URL = %q", release, toolchain.ArchiveURL)
		}
		if got := len(toolchain.SHA256); got != 64 {
			t.Errorf("Java %s SHA-256 length = %d", release, got)
		}
	}
}

func TestToolchainValidationRejectsBadDigest(t *testing.T) {
	t.Parallel()

	lock, err := oracle.LoadToolchainLock()
	if err != nil {
		t.Fatalf("LoadToolchainLock: %v", err)
	}
	toolchain := lock.Toolchains[0]
	toolchain.SHA256 = strings.Repeat("f", 63)
	if err := toolchain.Validate(); err == nil {
		t.Fatal("Validate with short digest returned nil error")
	}
}

func TestToolchainSelectionRejectsUnlockedTarget(t *testing.T) {
	t.Parallel()

	lock, err := oracle.LoadToolchainLock()
	if err != nil {
		t.Fatalf("LoadToolchainLock: %v", err)
	}
	if _, err := lock.Select(language.Release26, "linux", "amd64"); err == nil {
		t.Fatal("Select for unlocked target returned nil error")
	}
}
