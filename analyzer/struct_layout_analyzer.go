package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type StructLayoutAnalyzer struct{}

func NewStructLayoutAnalyzer() Analyzer {
	return &StructLayoutAnalyzer{}
}

func (s *StructLayoutAnalyzer) Name() string {
	return "StructLayout"
}

func (s *StructLayoutAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	info, _ := LoadTypes(fset, file, filename)
	if info == nil {
		return nil
	}

	issues := make([]*models.Issue, 0, 4)
	sizes := &types.StdSizes{WordSize: 8, MaxAlign: 8}

	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		if typeSpec.Name == nil {
			return true
		}

		obj := info.Defs[typeSpec.Name]
		if obj == nil {
			return true
		}

		named, ok := obj.Type().(*types.Named)
		if !ok {
			return true
		}

		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			return true
		}

		wasted := computePadding(st, sizes)
		if wasted >= 8 {
			pos := fset.Position(typeSpec.Pos())
			issues = append(issues, &models.Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueStructLayoutUnoptimized,
				Severity:   models.SeverityLevelLow,
				Message:    fmt.Sprintf("Struct %s wastes %d bytes due to padding", typeSpec.Name.Name, wasted),
				Suggestion: "Reorder fields to place larger types first and reduce padding",
			})
		}
		return true
	})

	return issues
}

func computePadding(st *types.Struct, sizes types.Sizes) int64 {
	var wasted int64
	var offset int64
	var maxAlign int64 = 1

	for i := 0; i < st.NumFields(); i++ {
		ft := st.Field(i).Type()
		size := sizes.Sizeof(ft)
		align := sizes.Alignof(ft)
		if align > maxAlign {
			maxAlign = align
		}
		padding := modPadding(offset, align)
		wasted += padding
		offset += padding + size
	}

	wasted += modPadding(offset, maxAlign)
	return wasted
}

func modPadding(offset, alignment int64) int64 {
	if alignment == 0 {
		return 0
	}
	remainder := offset % alignment
	if remainder == 0 {
		return 0
	}
	return alignment - remainder
}
