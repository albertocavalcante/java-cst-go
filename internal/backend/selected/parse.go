package selected

import (
	"context"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesittergo"
	"git.alberto.engineer/alberto/java-cst-go/language"
	"git.alberto.engineer/alberto/java-cst-go/source"
)

// Parse parses raw Java source with the repository's selected backend.
func Parse(
	ctx context.Context,
	input []byte,
	level language.Level,
) (backend.Result, error) {
	return treesittergo.Parse(ctx, input, level)
}

// ParseWithLimits parses raw Java source with explicit per-parse limits.
func ParseWithLimits(
	ctx context.Context,
	input []byte,
	level language.Level,
	limits backend.Limits,
) (backend.Result, error) {
	return treesittergo.ParseWithLimits(ctx, input, level, limits)
}

// ParseTranslation parses a Java Unicode-translated source stream with the
// repository's selected backend and projects its ranges back to raw source.
func ParseTranslation(
	ctx context.Context,
	translation *source.Translation,
	level language.Level,
) (backend.Result, error) {
	return treesittergo.ParseTranslation(ctx, translation, level)
}

// ParseTranslationWithLimits parses translated Java source with explicit
// per-parse limits.
func ParseTranslationWithLimits(
	ctx context.Context,
	translation *source.Translation,
	level language.Level,
	limits backend.Limits,
) (backend.Result, error) {
	return treesittergo.ParseTranslationWithLimits(ctx, translation, level, limits)
}
