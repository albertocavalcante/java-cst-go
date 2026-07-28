package backend_test

import (
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
)

func TestDefaultLimitsAreBounded(t *testing.T) {
	t.Parallel()

	limits := backend.DefaultLimits()
	if limits.MaxSourceBytes == 0 || limits.MaxNodes == 0 || limits.MaxDepth == 0 {
		t.Fatalf("DefaultLimits() = %+v, want positive bounds", limits)
	}
}

func TestResolveLimitsDefaultsFieldsIndependently(t *testing.T) {
	t.Parallel()

	limits := backend.ResolveLimits(backend.Limits{MaxNodes: 7})
	if limits.MaxSourceBytes != backend.DefaultMaxSourceBytes ||
		limits.MaxNodes != 7 ||
		limits.MaxDepth != backend.DefaultMaxDepth {
		t.Fatalf("ResolveLimits() = %+v", limits)
	}
}

func TestLimitErrorDescribesResource(t *testing.T) {
	t.Parallel()

	err := (&backend.LimitError{
		Kind:   backend.LimitNodes,
		Limit:  10,
		Actual: 11,
	}).Error()
	for _, text := range []string{"nodes", "10", "11"} {
		if !strings.Contains(err, text) {
			t.Errorf("LimitError = %q, want %q", err, text)
		}
	}
}
