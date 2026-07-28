// Package javacst parses Java source into lossless, immutable syntax trees.
package javacst

import (
	"context"
	"errors"
	"fmt"

	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/selected"
	"git.alberto.engineer/alberto/java-cst-go/internal/convert"
	"git.alberto.engineer/alberto/java-cst-go/internal/diagnose"
	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
	"git.alberto.engineer/alberto/java-cst-go/source"
	"git.alberto.engineer/alberto/java-cst-go/syntax"
)

const developmentVersion = "devel"

// Parse parses Java source using options.
func Parse(text string, options Options) (*syntax.Tree, error) {
	return ParseContext(context.Background(), text, options)
}

// ParseContext parses Java source using options and observes ctx cancellation.
//
// Java syntax and lexical errors are returned on the non-nil tree. The Go
// error reports only invalid configuration, cancellation, resource limits,
// or internal invariant failures.
func ParseContext(
	ctx context.Context,
	text string,
	options Options,
) (*syntax.Tree, error) {
	if ctx == nil {
		return nil, errors.New("parse Java source: nil context")
	}
	level, ok := options.resolveLevel()
	if !ok {
		return nil, fmt.Errorf(
			"parse Java source: invalid language level %q",
			options.Level,
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("parse Java source: %w", err)
	}

	limits := options.Limits.resolve()
	if uint64(len(text)) > uint64(limits.MaxSourceBytes) {
		return nil, fmt.Errorf("parse Java source: %w", &LimitError{
			Kind:   LimitSourceBytes,
			Limit:  uint64(limits.MaxSourceBytes),
			Actual: uint64(len(text)),
		})
	}
	translation := source.Translate(text)
	snapshot, err := selected.ParseTranslationWithLimits(
		ctx,
		translation,
		level,
		limits,
	)
	if err != nil {
		var limitErr *backend.LimitError
		if errors.As(err, &limitErr) {
			return nil, fmt.Errorf("parse Java source: %w", &LimitError{
				Kind:   LimitKind(limitErr.Kind),
				Limit:  limitErr.Limit,
				Actual: limitErr.Actual,
			})
		}
		return nil, fmt.Errorf("parse Java source: %w", err)
	}
	converted, err := convert.ConvertTranslation(translation, snapshot)
	if err != nil {
		return nil, fmt.Errorf("parse Java source: %w", err)
	}
	diagnostics := diagnostic.Normalize(
		translation.Diagnostics(),
		diagnose.Backend(snapshot),
	)
	tree, err := syntax.NewTree(
		converted.Tree,
		text,
		level,
		translation,
		diagnostics,
		syntax.Provenance{
			LibraryVersion:  developmentVersion,
			GrammarRevision: java25.GrammarRevision,
			Backend:         "treesitter-go/" + java25.RuntimeVersion,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("parse Java source: %w", err)
	}
	return tree, nil
}
