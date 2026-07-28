package oracle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Installer downloads, verifies, and extracts locked JDK archives.
type Installer struct {
	CacheDir string
	Client   *http.Client
}

// Installation identifies one verified cache entry.
type Installation struct {
	Root     string
	JavaHome string
	Javac    string
}

// DefaultCacheDir returns the per-user, non-system oracle cache directory.
func DefaultCacheDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}

	return filepath.Join(cache, "java-cst-go", "oracle"), nil
}

// Ensure returns an existing verified installation or installs toolchain.
func (installer Installer) Ensure(
	ctx context.Context,
	toolchain Toolchain,
) (Installation, error) {
	if err := toolchain.Validate(); err != nil {
		return Installation{}, fmt.Errorf("install JDK: %w", err)
	}
	if installer.CacheDir == "" {
		return Installation{}, errors.New("install JDK: cache directory is empty")
	}

	root := filepath.Join(
		installer.CacheDir,
		"toolchains",
		fmt.Sprintf(
			"openjdk-%s-%s-%s-%s",
			toolchain.Version,
			toolchain.OS,
			toolchain.Arch,
			toolchain.SHA256[:12],
		),
	)
	installation := installationAt(root, toolchain)
	if err := verifyInstallation(installation); err == nil {
		return installation, nil
	}

	archive, err := installer.ensureArchive(ctx, toolchain)
	if err != nil {
		return Installation{}, err
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Installation{}, fmt.Errorf("create JDK toolchain cache: %w", err)
	}
	temp, err := os.MkdirTemp(parent, ".install-*")
	if err != nil {
		return Installation{}, fmt.Errorf("create temporary JDK installation: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(temp)
	}()

	if err := extractTarGzip(archive, temp); err != nil {
		return Installation{}, fmt.Errorf("extract JDK archive: %w", err)
	}
	tempInstallation := installationAt(temp, toolchain)
	if err := verifyInstallation(tempInstallation); err != nil {
		return Installation{}, err
	}
	if err := os.Rename(temp, root); err != nil {
		if existingErr := verifyInstallation(installation); existingErr == nil {
			return installation, nil
		}
		return Installation{}, fmt.Errorf("publish JDK installation: %w", err)
	}

	return installation, nil
}

func (installer Installer) ensureArchive(
	ctx context.Context,
	toolchain Toolchain,
) (string, error) {
	directory := filepath.Join(installer.CacheDir, "archives")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create JDK archive cache: %w", err)
	}
	archive := filepath.Join(directory, toolchain.SHA256+".tar.gz")
	if _, err := os.Stat(archive); err == nil {
		if err := verifyArchive(archive, toolchain); err != nil {
			return "", fmt.Errorf("verify cached JDK archive: %w", err)
		}
		return archive, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect cached JDK archive: %w", err)
	}

	temp, err := os.CreateTemp(directory, ".download-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create temporary JDK archive: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, toolchain.ArchiveURL, nil)
	if err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("create JDK download request: %w", err)
	}
	client := installer.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		_ = temp.Close()
		return "", fmt.Errorf("download JDK archive: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		_ = temp.Close()
		return "", fmt.Errorf("download JDK archive: HTTP %s", response.Status)
	}
	if response.Request.URL.Scheme != "https" {
		_ = temp.Close()
		return "", fmt.Errorf(
			"download JDK archive: redirect ended at non-HTTPS URL %q",
			response.Request.URL,
		)
	}

	hash := sha256.New()
	written, copyErr := io.Copy(
		io.MultiWriter(temp, hash),
		io.LimitReader(response.Body, toolchain.ArchiveBytes+1),
	)
	closeErr := temp.Close()
	if copyErr != nil {
		return "", fmt.Errorf("download JDK archive body: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close JDK archive: %w", closeErr)
	}
	if written != toolchain.ArchiveBytes {
		return "", fmt.Errorf(
			"downloaded JDK archive is %d bytes, want %d",
			written,
			toolchain.ArchiveBytes,
		)
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != toolchain.SHA256 {
		return "", fmt.Errorf(
			"downloaded JDK archive SHA-256 = %s, want %s",
			got,
			toolchain.SHA256,
		)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return "", fmt.Errorf("set JDK archive permissions: %w", err)
	}
	if err := os.Rename(tempPath, archive); err != nil {
		if verifyErr := verifyArchive(archive, toolchain); verifyErr == nil {
			return archive, nil
		}
		return "", fmt.Errorf("publish JDK archive: %w", err)
	}

	return archive, nil
}

func verifyArchive(path string, toolchain Toolchain) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != toolchain.ArchiveBytes {
		return fmt.Errorf("size = %d, want %d", info.Size(), toolchain.ArchiveBytes)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != toolchain.SHA256 {
		return fmt.Errorf("SHA-256 = %s, want %s", got, toolchain.SHA256)
	}

	return nil
}

func installationAt(root string, toolchain Toolchain) Installation {
	javaHome := filepath.Join(root, filepath.FromSlash(toolchain.JavaHome))
	return Installation{
		Root:     root,
		JavaHome: javaHome,
		Javac:    filepath.Join(javaHome, "bin", "javac"),
	}
}

func verifyInstallation(installation Installation) error {
	info, err := os.Stat(installation.Javac)
	if err != nil {
		return fmt.Errorf("verify installed javac: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf(
			"verify installed javac: %q is not an executable regular file",
			installation.Javac,
		)
	}

	return nil
}

func extractTarGzip(archive string, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() {
		_ = compressed.Close()
	}()
	reader := tar.NewReader(compressed)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." && header.Typeflag == tar.TypeDir {
			continue
		}
		if clean == "." ||
			filepath.IsAbs(clean) ||
			clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		if err := ensureWithin(destination, target); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := ensureNoSymlinkParents(destination, target); err != nil {
				return err
			}
			if err := os.MkdirAll(target, os.FileMode(header.Mode)&0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := ensureNoSymlinkParents(destination, filepath.Dir(target)); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(
				target,
				os.O_CREATE|os.O_EXCL|os.O_WRONLY,
				os.FileMode(header.Mode)&0o755,
			)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(header.Linkname) {
				return fmt.Errorf("unsafe absolute symlink %q", header.Linkname)
			}
			resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(header.Linkname))
			if err := ensureWithin(destination, resolved); err != nil {
				return fmt.Errorf("unsafe symlink %q: %w", header.Name, err)
			}
			if err := ensureNoSymlinkParents(destination, filepath.Dir(target)); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(filepath.FromSlash(header.Linkname), target); err != nil {
				return err
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf(
				"unsupported archive entry %q with type %d",
				header.Name,
				header.Typeflag,
			)
		}
	}
}

func ensureNoSymlinkParents(root string, directory string) error {
	relative, err := filepath.Rel(root, directory)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "." || component == "" {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive path traverses symlink %q", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("archive parent %q is not a directory", current)
		}
	}

	return nil
}

func ensureWithin(root string, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if relative == ".." ||
		filepath.IsAbs(relative) ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q escapes extraction root", target)
	}

	return nil
}
