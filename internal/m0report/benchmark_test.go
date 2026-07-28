package m0report

import (
	"context"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend/treesitter"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func BenchmarkM0ParseAndConvert(b *testing.B) {
	cases := []struct {
		name   string
		source string
		level  language.Level
	}{
		{
			name: "java21-pattern-switch",
			source: `final class Example {
    static String describe(Object value) {
        return switch (value) {
            case String text when !text.isEmpty() -> text;
            default -> "other";
        };
    }
}
`,
			level: language.Level{Release: language.Release21},
		},
		{
			name: "java25-module-import-gap",
			source: `import module java.base;
final class Example {
    List<String> values = List.of("m0");
}
`,
			level: language.Level{Release: language.Release25},
		},
		{
			name:   "translated-identifiers",
			source: `class \u0045xample { int \u0076alue = 1\u003b }`,
			level:  language.Level{Release: language.Release21},
		},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(test.source)))
			for b.Loop() {
				if _, err := runPipeline(
					context.Background(),
					test.source,
					test.level,
					treesitter.ParseTranslation,
				); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
