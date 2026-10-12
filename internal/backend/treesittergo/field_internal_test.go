package treesittergo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dts "github.com/dcosson/treesitter-go"
	dparser "github.com/dcosson/treesitter-go/parser"

	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
)

// referenceField is the original O(depth) implementation, kept as an oracle.
func referenceField(cursor *dts.TreeCursor, lang *dts.Language) string {
	if len(cursor.Stack) < 2 {
		return ""
	}
	arena := cursor.Tree.Arena()
	field := ""
	for index := 1; index < len(cursor.Stack); index++ {
		parent := cursor.Stack[index-1]
		child := cursor.Stack[index]
		if dts.IsVisible(parent.Subtree, arena) {
			field = ""
		}
		if dts.IsExtra(child.Subtree, arena) {
			field = ""
			continue
		}
		entries := lang.FieldMapForProduction(dts.GetProductionID(parent.Subtree, arena))
		for _, e := range entries {
			if !e.Inherited && e.ChildIndex == uint16(child.StructuralChildIndex) {
				field = lang.FieldName(e.FieldID)
				break
			}
		}
	}
	return field
}

func TestCurrentFieldMatchesReference(t *testing.T) {
	corpus := []string{
		"class A{ int x=1; void f(int a,int b){ if(a>b){return;} else {x=a+b*2;} } }",
		"class A{" + strings.Repeat("int x=1;\n", 300) + "}",
		"interface I<T extends Number>{ default T g(T t){ return t; } } // c\n/* d */ enum E{A,B;}",
		"record R(int a,String b){} @interface Z{} class B{ int[] a={1,2,3}; var s=\"x\"+f(1,2).g; }",
		"@ # ` @ # ` class { ; int ( }}} {{",
		"class A{ void f(){ switch(x){ case 1 -> y(); default -> { } } try(var r=o()){}catch(E|F e){}finally{} } }",
	}
	files, _ := filepath.Glob("../../../testdata/*/*.java")
	files2, _ := filepath.Glob("../../../testdata/*/*/*.java")
	for _, f := range append(files, files2...) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		corpus = append(corpus, string(b))
	}
	corpus = append(corpus,
		strings.Repeat("@ # ` ", 300),
		"class A{"+strings.Repeat("{", 200),
		"class A{ void f(){"+strings.Repeat("{ int a=1; ", 100)+strings.Repeat("}", 100)+"} }",
		"class A{"+strings.Repeat("int x; /*c*/ void m(){} // e\n", 200)+"}",
	)
	lang := java25.JavaLanguage()
	fields := 0
	for _, src := range corpus {
		p := dparser.NewParser()
		p.SetLanguage(lang)
		tree := p.ParseString(context.Background(), []byte(src))
		cursor := dts.NewTreeCursor(tree.RootNode())
		// One tracker per walk, driven in snapshotCursor's move order.
		tracker := newFieldTracker(lang)
		var walk func()
		walk = func() {
			got, want := tracker.sync(&cursor), referenceField(&cursor, lang)
			if got != want {
				t.Fatalf("field = %q, want %q at %s", got, want, cursor.CurrentNode().Type())
			}
			if want != "" {
				fields++
			}
			if !cursor.GotoFirstChild() {
				return
			}
			for {
				walk()
				if !cursor.GotoNextSibling() {
					break
				}
			}
			cursor.GotoParent()
		}
		walk()
	}
	if fields == 0 {
		t.Fatal("corpus exercised no fields")
	}
}

func TestCurrentFieldFromScratchMatchesReference(t *testing.T) {
	lang := java25.JavaLanguage()
	p := dparser.NewParser()
	p.SetLanguage(lang)
	tree := p.ParseString(context.Background(), []byte("class A{ int x=1; void f(){ g(1,2); } }"))
	cursor := dts.NewTreeCursor(tree.RootNode())
	for cursor.GotoFirstChild() {
		if got, want := currentField(&cursor, lang), referenceField(&cursor, lang); got != want {
			t.Fatalf("field = %q, want %q", got, want)
		}
	}
}
