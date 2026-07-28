package diagnose

import (
	"git.alberto.engineer/alberto/java-cst-go/diagnostic"
	"git.alberto.engineer/alberto/java-cst-go/internal/backend"
)

// Backend converts explicit backend recovery nodes into stable diagnostics.
func Backend(result backend.Result) []diagnostic.Diagnostic {
	if result.Root == nil {
		return nil
	}

	values := make([]diagnostic.Diagnostic, 0, result.ErrorCount+result.MissingCount)
	stack := []*backend.Node{result.Root}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]

		span := diagnostic.Span{
			Start: int(node.StartByte),
			End:   int(node.EndByte),
		}
		if node.Error {
			values = append(values, diagnostic.NewSource(
				diagnostic.CodeUnexpectedToken,
				diagnostic.SeverityError,
				span,
				"unexpected Java syntax",
			))
		}
		if node.Missing {
			values = append(values, diagnostic.NewSource(
				diagnostic.CodeMissingToken,
				diagnostic.SeverityError,
				span,
				"expected Java syntax is missing",
			))
		}

		for index := len(node.Children) - 1; index >= 0; index-- {
			stack = append(stack, &node.Children[index])
		}
	}
	return diagnostic.Normalize(values)
}
