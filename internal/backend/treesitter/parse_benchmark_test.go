package treesitter_test

import (
	"context"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

// BenchmarkPathologicalRecoveryUnbounded deliberately omits the production
// deadline so runtime regressions remain measurable. Run it with an explicit,
// small benchtime; it is not part of the normal test gate.
func BenchmarkPathologicalRecoveryUnbounded(b *testing.B) {
	source := []byte("#0#.}")
	level := language.Level{Release: language.Release21}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	var stopReason string
	var stoppedEarly bool
	for range b.N {
		result, err := treesitter.Parse(context.Background(), source, level)
		if err != nil {
			b.Fatal(err)
		}
		stopReason = result.StopReason
		stoppedEarly = result.StoppedEarly
	}
	b.StopTimer()
	b.Logf("stop_reason=%s stopped_early=%t", stopReason, stoppedEarly)
}
