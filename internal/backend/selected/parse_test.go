package selected_test

import (
	"context"
	"strings"
	"testing"

	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend/selected"
	"git.alberto.engineer/alberto/java-cst-go/language"
)

func TestParseUsesPatchedJava25Backend(t *testing.T) {
	t.Parallel()

	source := []byte(`import module java.base;
class Base {}
class Example extends Base {
    Example() {
        int before = 1;
        super();
    }
}
`)
	result, err := selected.Parse(
		context.Background(),
		source,
		language.Level{Release: language.Release25},
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.HasPrefix(result.Backend, "treesitter-go/v0.1.0:") {
		t.Fatalf("backend = %q, want selected treesitter-go runtime", result.Backend)
	}
	if result.Root == nil || result.Root.HasError || result.ErrorCount != 0 {
		t.Fatalf("selected backend result = %+v, want clean tree", result)
	}
	for _, kind := range []string{
		"module_import_declaration",
		"explicit_constructor_invocation",
	} {
		if !hasKind(result.Root, kind) {
			t.Errorf("selected backend tree has no %q node", kind)
		}
	}
}

func hasKind(root *backend.Node, kind string) bool {
	stack := []*backend.Node{root}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if node.Kind == kind {
			return true
		}
		for index := range node.Children {
			stack = append(stack, &node.Children[index])
		}
	}
	return false
}
