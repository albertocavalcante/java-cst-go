package testkit

import (
	"encoding/json"
	"fmt"
	"io"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

// Fixture describes one versioned M0 source probe.
type Fixture struct {
	ID                string           `json:"id"`
	Path              string           `json:"path"`
	Release           language.Release `json:"release"`
	Preview           bool             `json:"preview"`
	Category          string           `json:"category"`
	ExpectedBackend   string           `json:"expectedBackend"`
	ExpectedRoundTrip bool             `json:"expectedRoundTrip"`
}

// Manifest is the checked-in set of M0 fixture probes.
type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	Fixtures      []Fixture `json:"fixtures"`
}

// DecodeManifest decodes and validates a fixture manifest.
func DecodeManifest(reader io.Reader) (Manifest, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode M0 fixture manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return Manifest{}, fmt.Errorf(
			"M0 fixture schema version = %d, want 1",
			manifest.SchemaVersion,
		)
	}

	seen := make(map[string]struct{}, len(manifest.Fixtures))
	for index, fixture := range manifest.Fixtures {
		if fixture.ID == "" {
			return Manifest{}, fmt.Errorf("M0 fixture %d has an empty ID", index)
		}
		if _, ok := seen[fixture.ID]; ok {
			return Manifest{}, fmt.Errorf("duplicate M0 fixture ID %q", fixture.ID)
		}
		seen[fixture.ID] = struct{}{}

		if fixture.Path == "" {
			return Manifest{}, fmt.Errorf("M0 fixture %q has an empty path", fixture.ID)
		}
		if !fixture.Release.Valid() {
			return Manifest{}, fmt.Errorf(
				"M0 fixture %q has invalid Java release %d",
				fixture.ID,
				fixture.Release,
			)
		}
		if fixture.ExpectedBackend != "measure" {
			return Manifest{}, fmt.Errorf(
				"M0 fixture %q expectedBackend = %q, want %q",
				fixture.ID,
				fixture.ExpectedBackend,
				"measure",
			)
		}
	}

	return manifest, nil
}
