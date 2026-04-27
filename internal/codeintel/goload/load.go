package goload

import (
	"fmt"
	"go/token"
	goTypes "go/types"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

type InterfaceInfo struct {
	FullName string
	PkgPath  string
	Name     string
	Type     *goTypes.Interface
}

type Result struct {
	FileSet    *token.FileSet
	Packages   []*packages.Package
	ByFile     map[string]*packages.Package
	Interfaces []InterfaceInfo
}

func Load(rootDir string, modulePath string) (*Result, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps |
			packages.NeedImports,
		Dir:  rootDir,
		Fset: fset,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("go/packages load: %w", err)
	}

	for _, pkg := range pkgs {
		for _, e := range pkg.Errors {
			slog.Warn("go/packages error", "pkg", pkg.PkgPath, "error", e)
		}
	}

	result := &Result{
		FileSet: fset,
		ByFile:  make(map[string]*packages.Package),
	}

	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkg.Types == nil {
			return true
		}
		if modulePath != "" && !strings.HasPrefix(pkg.PkgPath, modulePath) {
			return true
		}

		for _, f := range pkg.GoFiles {
			abs, err := filepath.Abs(f)
			if err == nil {
				result.ByFile[abs] = pkg
			}
		}

		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			tn, ok := obj.(*goTypes.TypeName)
			if !ok {
				continue
			}
			iface, ok := tn.Type().Underlying().(*goTypes.Interface)
			if !ok || iface.NumMethods() == 0 {
				continue
			}
			result.Interfaces = append(result.Interfaces, InterfaceInfo{
				FullName: pkg.PkgPath + "." + name,
				PkgPath:  pkg.PkgPath,
				Name:     name,
				Type:     iface,
			})
		}
		return true
	}, nil)

	result.Packages = pkgs
	return result, nil
}
