package gendocs

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadedPackages holds parsed Go packages keyed by import path.
type LoadedPackages struct {
	pkgs map[string]*packages.Package
}

// LoadPackages loads the Go packages referenced by the registry targets.
// It uses syntax-level loading (AST) without type-checking, so it never
// imports integration binaries.
func LoadPackages(root string, reg *Registry) (*LoadedPackages, error) {
	// Collect unique package paths.
	pkgSet := make(map[string]bool)
	for _, t := range reg.Targets {
		if t.Package != "" {
			pkgSet[t.Package] = true
		}
		for _, pt := range t.Types {
			if pt.Package != "" {
				pkgSet[pt.Package] = true
			}
		}
	}

	patterns := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		patterns = append(patterns, p)
	}

	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedFiles | packages.NeedName,
		Dir:  root,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}

	loaded := &LoadedPackages{pkgs: make(map[string]*packages.Package, len(pkgs))}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		loaded.pkgs[pkg.PkgPath] = pkg
	}

	return loaded, nil
}

// StructInfo holds parsed information about a Go struct.
type StructInfo struct {
	Name   string
	Doc    string
	Fields []FieldInfo
}

// FieldInfo holds parsed information about a single struct field.
type FieldInfo struct {
	Name    string
	GoType  string
	Doc     string
	HCLTag  *HCLTag
}

// FindStruct locates a struct by name in a loaded package and extracts
// its field info (types, tags, doc comments).
func (lp *LoadedPackages) FindStruct(pkgPath, typeName string) (*StructInfo, error) {
	pkg, ok := lp.pkgs[pkgPath]
	if !ok {
		return nil, fmt.Errorf("package %s not loaded", pkgPath)
	}

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != typeName {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					return nil, fmt.Errorf("%s.%s is not a struct", pkgPath, typeName)
				}

				var doc string
				if genDecl.Doc != nil && len(genDecl.Specs) == 1 {
					doc = CleanDoc(genDecl.Doc.Text())
				} else if typeSpec.Doc != nil {
					doc = CleanDoc(typeSpec.Doc.Text())
				}

				info := &StructInfo{
					Name: typeName,
					Doc:  doc,
				}

				for _, field := range structType.Fields.List {
					if len(field.Names) == 0 {
						continue // embedded field
					}

					fi := FieldInfo{
						Name:   field.Names[0].Name,
						GoType: formatTypeExpr(field.Type),
					}

					// Parse doc comment
					if field.Doc != nil {
						fi.Doc = CleanDoc(field.Doc.Text())
					} else if field.Comment != nil {
						fi.Doc = CleanDoc(field.Comment.Text())
					}

					// Parse hcl tag
					if field.Tag != nil {
						tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
						hclVal := tag.Get("hcl")
						if hclVal != "" {
							fi.HCLTag = ParseHCLTag(hclVal)
						}
					}

					info.Fields = append(info.Fields, fi)
				}

				return info, nil
			}
		}
	}

	return nil, fmt.Errorf("struct %s.%s not found", pkgPath, typeName)
}

// formatTypeExpr converts an AST type expression to a human-readable string.
func formatTypeExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + formatTypeExpr(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + formatTypeExpr(t.Elt)
		}
		return "[...]" + formatTypeExpr(t.Elt)
	case *ast.MapType:
		return "map[" + formatTypeExpr(t.Key) + "]" + formatTypeExpr(t.Value)
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.InterfaceType:
		return "any"
	default:
		return "unknown"
	}
}
