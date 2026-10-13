package treesittergo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesittergo"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestParseLongMemberListIsNearLinear(t *testing.T) {
	if testing.Short() {
		t.Skip("wall-clock scaling test skipped in -short mode")
	}
	if raceEnabled {
		t.Skip("wall-clock scaling test skipped under the race detector")
	}
	level := language.Level{Release: language.Release21}
	parse := func(n int) time.Duration {
		src := []byte("class A{" + strings.Repeat("int x=1;\n", n) + "}")
		start := time.Now()
		result, err := treesittergo.Parse(context.Background(), src, level)
		if err != nil || result.Root == nil || result.ErrorCount != 0 {
			t.Fatalf("n=%d: err=%v result=%+v", n, err, result)
		}
		return time.Since(start)
	}
	small := parse(5000)
	large := parse(20000)
	// Linear growth predicts 4x; allow generous slack plus a hard ceiling.
	if large > 8*small+500*time.Millisecond || large > 5*time.Second {
		t.Fatalf("parse(20000)=%s vs parse(5000)=%s: super-linear", large, small)
	}
}
