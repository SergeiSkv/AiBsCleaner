package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

func ParseFile(filename string) (*ast.File, *token.FileSet, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	return file, fset, nil
}

// ParseCode parses Go code from a string
func ParseCode(code string, filename string) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, code, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	return file, fset, nil
}
