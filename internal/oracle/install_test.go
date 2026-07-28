package oracle_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestInstallerVerifiesAndReusesArchive(t *testing.T) {
	t.Parallel()

	archive := makeArchive(t, []archiveEntry{{
		name: "./",
		mode: 0o755,
	}, {
		name: "fake.jdk/Contents/Home/bin/javac",
		mode: 0o755,
		body: "#!/bin/sh\n",
	}})
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		requests.Add(1)
		writer.WriteHeader(http.StatusOK)
		if _, err := writer.Write(archive); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	toolchain := fakeToolchain(server.URL, archive)
	installer := oracle.Installer{
		CacheDir: t.TempDir(),
		Client:   server.Client(),
	}
	first, err := installer.Ensure(context.Background(), toolchain)
	if err != nil {
		t.Fatalf("first Ensure: %v", err)
	}
	if _, err := os.Stat(first.Javac); err != nil {
		t.Fatalf("stat installed javac: %v", err)
	}
	second, err := installer.Ensure(context.Background(), toolchain)
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if first != second {
		t.Fatalf("installations differ: first=%+v second=%+v", first, second)
	}
	if got, want := requests.Load(), int32(1); got != want {
		t.Fatalf("download requests = %d, want %d", got, want)
	}
}

func TestInstallerRejectsChecksumMismatchBeforeExtraction(t *testing.T) {
	t.Parallel()

	archive := makeArchive(t, []archiveEntry{{
		name: "fake.jdk/Contents/Home/bin/javac",
		mode: 0o755,
		body: "not executed",
	}})
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		if _, err := writer.Write(archive); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	toolchain := fakeToolchain(server.URL, archive)
	toolchain.SHA256 = strings.Repeat("0", 64)
	cache := t.TempDir()
	_, err := (oracle.Installer{
		CacheDir: cache,
		Client:   server.Client(),
	}).Ensure(context.Background(), toolchain)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Ensure error = %v, want SHA-256 mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "toolchains")); !os.IsNotExist(err) {
		t.Fatalf("toolchain directory exists after mismatch: %v", err)
	}
}

func TestInstallerRejectsArchiveTraversal(t *testing.T) {
	t.Parallel()

	archive := makeArchive(t, []archiveEntry{{
		name: "../escape",
		mode: 0o644,
		body: "escape",
	}})
	server := httptest.NewTLSServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		if _, err := writer.Write(archive); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	toolchain := fakeToolchain(server.URL, archive)
	cache := t.TempDir()
	_, err := (oracle.Installer{
		CacheDir: cache,
		Client:   server.Client(),
	}).Ensure(context.Background(), toolchain)
	if err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("Ensure error = %v, want unsafe archive path", err)
	}
	if _, err := os.Stat(filepath.Join(cache, "escape")); !os.IsNotExist(err) {
		t.Fatalf("escape path exists: %v", err)
	}
}

type archiveEntry struct {
	name string
	mode int64
	body string
}

func makeArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	for _, entry := range entries {
		entryType := byte(tar.TypeReg)
		if strings.HasSuffix(entry.name, "/") {
			entryType = tar.TypeDir
		}
		if err := writer.WriteHeader(&tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: entryType,
		}); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buffer.Bytes()
}

func fakeToolchain(url string, archive []byte) oracle.Toolchain {
	digest := sha256.Sum256(archive)
	return oracle.Toolchain{
		Release:      language.Release26,
		Version:      "26-test",
		Build:        "26-test+1",
		OS:           "darwin",
		Arch:         "arm64",
		ArchiveURL:   url,
		ArchiveBytes: int64(len(archive)),
		SHA256:       hex.EncodeToString(digest[:]),
		JavaHome:     "fake.jdk/Contents/Home",
	}
}
