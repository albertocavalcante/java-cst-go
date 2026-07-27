package language

import (
	"fmt"
	"strconv"
)

// Release is a concrete Java language release.
//
// The zero value is invalid. Values currently cover the M0 and v1 target,
// Java 8 through Java 26.
type Release uint8

const (
	Release8 Release = 8 + iota
	Release9
	Release10
	Release11
	Release12
	Release13
	Release14
	Release15
	Release16
	Release17
	Release18
	Release19
	Release20
	Release21
	Release22
	Release23
	Release24
	Release25
	Release26
)

// Level selects one concrete Java grammar and its release-specific preview
// mode.
type Level struct {
	Release Release
	Preview bool
}

// Valid reports whether r is in the implemented release-value range.
func (r Release) Valid() bool {
	return r >= Release8 && r <= Release26
}

// String returns the canonical decimal spelling of r, or "invalid" for an
// invalid value.
func (r Release) String() string {
	if !r.Valid() {
		return "invalid"
	}

	return strconv.FormatUint(uint64(r), 10)
}

// ParseRelease parses a canonical Java release. "1.8" is accepted as an alias
// for Java 8.
func ParseRelease(text string) (Release, error) {
	if text == "1.8" {
		return Release8, nil
	}

	number, err := strconv.ParseUint(text, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("parse Java release %q: %w", text, err)
	}

	release := Release(number)
	if !release.Valid() {
		return 0, fmt.Errorf("unsupported Java release %q: want 8 through 26", text)
	}

	return release, nil
}

// Valid reports whether l names a concrete release. Preview availability is
// validated separately because it depends on implemented feature variants.
func (l Level) Valid() bool {
	return l.Release.Valid()
}

// String returns the canonical level spelling.
func (l Level) String() string {
	if !l.Valid() {
		return "invalid"
	}
	if l.Preview {
		return l.Release.String() + "-preview"
	}

	return l.Release.String()
}
