// Command jdkoracle installs one checksum-verified compiler oracle.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	"git.alberto.engineer/alberto/java-cst-go/internal/oracle"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	releaseNumber := flag.Int("release", 26, "Java compiler release to install")
	cache := flag.String("cache", "", "oracle cache directory")
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

	fmt.Fprintf(
		os.Stderr,
		"ensuring OpenJDK %s (%s) in %s\n",
		toolchain.Version,
		toolchain.SHA256[:12],
		*cache,
	)
	installation, err := (oracle.Installer{CacheDir: *cache}).Ensure(
		context.Background(),
		toolchain,
	)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(os.Stdout, installation.JavaHome); err != nil {
		return fmt.Errorf("write Java home: %w", err)
	}

	return nil
}
