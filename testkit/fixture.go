package testkit

import (
	"encoding/json"
	"fmt"
	"io"

	"git.alberto.engineer/alberto/java-cst-go/language"
)

// Fixture describes one versioned M0 source probe.
type Fixture struct {
	ID                     string           `json:"id"`
	Path                   string           `json:"path"`
	Release                language.Release `json:"release"`
	Preview                bool             `json:"preview"`
	Category               string           `json:"category"`
	Feature                string           `json:"feature,omitempty"`
	ExpectedFeatureState   string           `json:"expectedFeatureState,omitempty"`
	ExpectedFeatureVariant uint8            `json:"expectedFeatureVariant,omitempty"`
	ExpectedFeatureEnabled bool             `json:"expectedFeatureEnabled,omitempty"`
	ExpectedBackend        string           `json:"expectedBackend"`
	ExpectedRoundTrip      bool             `json:"expectedRoundTrip"`
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
	if manifest.SchemaVersion != 2 {
		return Manifest{}, fmt.Errorf(
			"M0 fixture schema version = %d, want 2",
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
		switch fixture.Category {
		case "release-anchor":
			if fixture.Feature != "" {
				return Manifest{}, fmt.Errorf(
					"M0 release anchor %q has feature %q",
					fixture.ID,
					fixture.Feature,
				)
			}
		case "feature-boundary":
			if err := validateFeatureBoundary(fixture); err != nil {
				return Manifest{}, err
			}
		default:
			return Manifest{}, fmt.Errorf(
				"M0 fixture %q has invalid category %q",
				fixture.ID,
				fixture.Category,
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

func validateFeatureBoundary(fixture Fixture) error {
	feature, err := language.ParseFeatureID(fixture.Feature)
	if err != nil {
		return fmt.Errorf("M0 fixture %q: %w", fixture.ID, err)
	}

	level := language.Level{Release: fixture.Release, Preview: fixture.Preview}
	support := level.Feature(feature)
	if fixture.ExpectedFeatureState != support.State.String() {
		return fmt.Errorf(
			"M0 fixture %q feature state = %q, registry has %q",
			fixture.ID,
			fixture.ExpectedFeatureState,
			support.State,
		)
	}
	if fixture.ExpectedFeatureVariant != support.Variant {
		return fmt.Errorf(
			"M0 fixture %q feature variant = %d, registry has %d",
			fixture.ID,
			fixture.ExpectedFeatureVariant,
			support.Variant,
		)
	}
	if fixture.ExpectedFeatureEnabled != level.Supports(feature) {
		return fmt.Errorf(
			"M0 fixture %q feature enabled = %t, registry has %t",
			fixture.ID,
			fixture.ExpectedFeatureEnabled,
			level.Supports(feature),
		)
	}

	return nil
}
