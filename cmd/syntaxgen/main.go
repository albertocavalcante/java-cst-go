// Command syntaxgen generates Java syntax kinds and typed red views.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	flag.Parse()

	if *schemaPath == "" || *syntaxPath == "" || *astPath == "" {
		return errors.New("-schema, -syntax, and -ast are required")
	}
	schema, err := syntaxgen.Load(*schemaPath)
	if err != nil {
		return err
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
