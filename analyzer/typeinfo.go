package analyzer

import (
	"errors"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

var errNoTypesInfo = errors.New("no type information returned")

// LoadTypes performs type-checking for the file identified by filename.
// It attempts to load package information via go/packages and falls back to
// single-file type checking if necessary.
func LoadTypes(fset *token.FileSet, file *ast.File, filename string) (*types.Info, error) {
	if filename != "" {
		info, err := loadTypesWithPackages(fset, filename)
		switch {
		case info != nil:
			return info, err
		case err != nil && !errors.Is(err, errNoTypesInfo):
			return nil, err
		}
	}
	return loadTypesSingleFile(fset, file)
}

func loadTypesWithPackages(fset *token.FileSet, filename string) (*types.Info, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		abs = filename
	}

	cfg := &packages.Config{
		Fset: fset,
		Mode: packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, "file="+abs)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 || pkgs[0].TypesInfo == nil {
		return nil, errNoTypesInfo
	}
	return pkgs[0].TypesInfo, nil
}

func loadTypesSingleFile(fset *token.FileSet, file *ast.File) (*types.Info, error) {
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}

	conf := types.Config{
		FakeImportC: true,
		Importer:    importer.Default(),
	}
	_, err := conf.Check(file.Name.Name, fset, []*ast.File{file}, info)
	return info, err
}
