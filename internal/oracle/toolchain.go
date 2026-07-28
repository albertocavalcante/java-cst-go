// Package oracle provides the reproducible compiler oracle used by the M0
// backend decision. It is not part of the Java parser's runtime surface.
package oracle

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

//go:embed toolchains.lock.json
var lockFiles embed.FS

// ToolchainLock records the compiler archives used by the oracle matrix.
type ToolchainLock struct {
	SchemaVersion int         `json:"schemaVersion"`
	CatalogURL    string      `json:"catalogURL"`
	License       string      `json:"license"`
	Toolchains    []Toolchain `json:"toolchains"`
}

// Toolchain identifies one immutable OpenJDK compiler archive.
type Toolchain struct {
	Release      language.Release `json:"release"`
	Version      string           `json:"version"`
	Build        string           `json:"build"`
	OS           string           `json:"os"`
	Arch         string           `json:"arch"`
	ArchiveURL   string           `json:"archiveURL"`
	ArchiveBytes int64            `json:"archiveBytes"`
	SHA256       string           `json:"sha256"`
	JavaHome     string           `json:"javaHome"`
}

// LoadToolchainLock reads and validates the embedded compiler lock.
func LoadToolchainLock() (ToolchainLock, error) {
	data, err := lockFiles.ReadFile("toolchains.lock.json")
	if err != nil {
		return ToolchainLock{}, fmt.Errorf("read embedded JDK toolchain lock: %w", err)
	}

	var lock ToolchainLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return ToolchainLock{}, fmt.Errorf("decode JDK toolchain lock: %w", err)
	}
	if err := lock.Validate(); err != nil {
		return ToolchainLock{}, err
	}

	return lock, nil
}

// Validate checks the lock's provenance and every toolchain identity.
func (lock ToolchainLock) Validate() error {
	if lock.SchemaVersion != 1 {
		return fmt.Errorf("JDK toolchain schema version = %d, want 1", lock.SchemaVersion)
	}
	if err := validateHTTPSURL(lock.CatalogURL); err != nil {
		return fmt.Errorf("JDK catalog URL: %w", err)
	}
	if lock.License == "" {
		return errors.New("JDK toolchain lock: license is empty")
	}
	if len(lock.Toolchains) == 0 {
		return errors.New("JDK toolchain lock: no toolchains")
	}

	seen := make(map[string]struct{}, len(lock.Toolchains))
	for index, toolchain := range lock.Toolchains {
		if err := toolchain.Validate(); err != nil {
			return fmt.Errorf("JDK toolchain %d: %w", index, err)
		}
		key := fmt.Sprintf("%d/%s/%s", toolchain.Release, toolchain.OS, toolchain.Arch)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("JDK toolchain lock: duplicate target %s", key)
		}
		seen[key] = struct{}{}
	}

	return nil
}

// Validate checks one compiler archive identity.
func (toolchain Toolchain) Validate() error {
	switch {
	case !toolchain.Release.Valid():
		return fmt.Errorf("invalid release %d", toolchain.Release)
	case toolchain.Version == "":
		return errors.New("version is empty")
	case toolchain.Build == "":
		return errors.New("build is empty")
	case toolchain.OS == "":
		return errors.New("OS is empty")
	case toolchain.Arch == "":
		return errors.New("architecture is empty")
	case toolchain.ArchiveBytes <= 0:
		return fmt.Errorf("archive size = %d, want positive", toolchain.ArchiveBytes)
	}
	if err := validateHTTPSURL(toolchain.ArchiveURL); err != nil {
		return fmt.Errorf("archive URL: %w", err)
	}
	digest, err := hex.DecodeString(toolchain.SHA256)
	if err != nil || len(digest) != sha256.Size || toolchain.SHA256 != strings.ToLower(toolchain.SHA256) {
		return errors.New("SHA-256 is not 64 lowercase hexadecimal characters")
	}
	if toolchain.JavaHome == "" ||
		path.IsAbs(toolchain.JavaHome) ||
		path.Clean(toolchain.JavaHome) != toolchain.JavaHome ||
		toolchain.JavaHome == ".." ||
		strings.HasPrefix(toolchain.JavaHome, "../") {
		return fmt.Errorf("unsafe Java home %q", toolchain.JavaHome)
	}

	return nil
}

// Select returns the exact compiler for a release and host target.
func (lock ToolchainLock) Select(
	release language.Release,
	goos string,
	arch string,
) (Toolchain, error) {
	for _, toolchain := range lock.Toolchains {
		if toolchain.Release == release &&
			toolchain.OS == goos &&
			toolchain.Arch == arch {
			return toolchain, nil
		}
	}

	return Toolchain{}, fmt.Errorf(
		"no locked JDK for Java %s on %s/%s",
		release,
		goos,
		arch,
	)
}

func validateHTTPSURL(text string) error {
	parsed, err := url.Parse(text)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%q is not an absolute HTTPS URL", text)
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("%q contains forbidden URL components", text)
	}

	return nil
}
