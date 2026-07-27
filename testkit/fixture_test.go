package testkit_test

import (
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/testkit"
)

func TestDecodeManifest(t *testing.T) {
	t.Parallel()

	manifest, err := testkit.DecodeManifest(strings.NewReader(`{
	  "schemaVersion": 2,
	  "fixtures": [{
	    "id": "java8/lambda",
	    "path": "java8/lambda.java",
	    "release": 8,
	    "preview": false,
	    "category": "release-anchor",
	    "expectedBackend": "measure",
	    "expectedRoundTrip": true
	  }]
	}`))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if got, want := len(manifest.Fixtures), 1; got != want {
		t.Fatalf("fixture count = %d, want %d", got, want)
	}
}

func TestDecodeManifestRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	_, err := testkit.DecodeManifest(strings.NewReader(`{
	  "schemaVersion": 2,
	  "fixtures": [
	    {
	      "id": "duplicate",
	      "path": "one.java",
	      "release": 8,
	      "expectedBackend": "measure"
	    },
	    {
	      "id": "duplicate",
	      "path": "two.java",
	      "release": 11,
	      "expectedBackend": "measure"
	    }
	  ]
	}`))
	if err == nil {
		t.Fatal("DecodeManifest with duplicate IDs returned nil error")
	}
}

func TestDecodeManifestValidatesFeatureBoundary(t *testing.T) {
	t.Parallel()

	manifest, err := testkit.DecodeManifest(strings.NewReader(`{
	  "schemaVersion": 2,
	  "fixtures": [{
	    "id": "java23/module-import/preview-p1",
	    "path": "features/module-imports/basic.java",
	    "release": 23,
	    "preview": true,
	    "category": "feature-boundary",
	    "feature": "module-imports",
	    "expectedFeatureState": "preview",
	    "expectedFeatureVariant": 1,
	    "expectedFeatureEnabled": true,
	    "expectedBackend": "measure",
	    "expectedRoundTrip": true
	  }]
	}`))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if got, want := manifest.Fixtures[0].Feature, "module-imports"; got != want {
		t.Fatalf("feature = %q, want %q", got, want)
	}
}

func TestDecodeManifestRejectsFeatureRegistryMismatch(t *testing.T) {
	t.Parallel()

	_, err := testkit.DecodeManifest(strings.NewReader(`{
	  "schemaVersion": 2,
	  "fixtures": [{
	    "id": "java23/module-import/wrong",
	    "path": "features/module-imports/basic.java",
	    "release": 23,
	    "preview": false,
	    "category": "feature-boundary",
	    "feature": "module-imports",
	    "expectedFeatureState": "final",
	    "expectedFeatureVariant": 1,
	    "expectedFeatureEnabled": true,
	    "expectedBackend": "measure",
	    "expectedRoundTrip": true
	  }]
	}`))
	if err == nil {
		t.Fatal("DecodeManifest with mismatched feature state returned nil error")
	}
}
