package treesittergo_test

import (
	"context"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesittergo"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func BenchmarkPathologicalRecovery(b *testing.B) {
	source := []byte("#0#.}")
	level := language.Level{Release: language.Release21}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()
	for range b.N {
		result, err := treesittergo.Parse(context.Background(), source, level)
		if err != nil {
			b.Fatal(err)
		}
		if result.StoppedEarly || result.ErrorCount == 0 {
			b.Fatalf("incomplete recovery result: %+v", result)
		}
	}
}
