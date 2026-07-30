// Command syntaxgen generates Java syntax kinds and typed red views.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"git.alberto.engineer/alberto/java-cst-go/internal/grammar/java25"
	"git.alberto.engineer/alberto/java-cst-go/internal/syntaxgen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	schemaPath := flag.String("schema", "", "path to java-syntax.json")
	syntaxPath := flag.String("syntax", "", "output path for generated syntax kinds")
	astPath := flag.String("ast", "", "output path for generated typed views")
	syncKinds := flag.Bool(
		"sync-grammar-kinds",
		false,
		"append missing selected-grammar kinds to the schema",
	)
	flag.Parse()

	if *schemaPath == "" {
		return errors.New("-schema is required")
	}
	schema, err := syntaxgen.Load(*schemaPath)
	if err != nil {
		return err
	}
	if *syncKinds {
		if err := appendGrammarKinds(&schema); err != nil {
			return err
		}
		data, err := syntaxgen.Marshal(schema)
		if err != nil {
			return err
		}
		return writeFile(*schemaPath, data)
	}
	if *syntaxPath == "" || *astPath == "" {
		return errors.New("-syntax and -ast are required when not syncing kinds")
	}
	syntaxSource, astSource, err := syntaxgen.Generate(schema)
	if err != nil {
		return err
	}
	if err := writeFile(*syntaxPath, syntaxSource); err != nil {
		return fmt.Errorf("write syntax kinds: %w", err)
	}
	if err := writeFile(*astPath, astSource); err != nil {
		return fmt.Errorf("write typed views: %w", err)
	}
	return nil
}

func appendGrammarKinds(schema *syntaxgen.Schema) error {
	if err := schema.AppendKind("node", "ERROR"); err != nil {
		return err
	}
	if err := schema.AppendKind("token", "ERROR"); err != nil {
		return err
	}
	if err := schema.AppendKind("token", "eof"); err != nil {
		return err
	}

	grammar := java25.JavaLanguage()
	for index, name := range grammar.SymbolNames {
		switch {
		case uint32(index) < grammar.TokenCount:
			if err := schema.AppendKind("token", name); err != nil {
				return err
			}
		case uint32(index) < grammar.SymbolCount:
			if err := schema.AppendKind("node", name); err != nil {
				return err
			}
		default:
			if err := schema.AppendKind("node", name); err != nil {
				return err
			}
			if err := schema.AppendKind("token", name); err != nil {
				return err
			}
		}
	}
	for _, name := range java25.CollapsedLeafKinds() {
		if err := schema.AppendKind("token", name); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".syntaxgen-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
