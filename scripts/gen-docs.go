//go:build ignore

// gen-docs generates JSON schema documentation from Go struct definitions.
// It parses apis/v1/*.go files and outputs JSON files to website/src/data/schemas/.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

type Schema struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

type Field struct {
	Name        string   `json:"name"`
	YAMLKey     string   `json:"yamlKey"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Template    bool     `json:"template"`
	Description string   `json:"description"`
	Enum        []string `json:"enum"`
	Ref         *string  `json:"ref"`
	Default     *string  `json:"default"`
}

// Structs to generate documentation for, mapped to output filenames
var targetStructs = map[string]string{
	// Collectors
	"TerraformCollector": "terraform-collector.json",
	"HTTPCollector":      "http-collector.json",
	"HTTPAuth":           "http-auth.json",
	"HTTPBasicAuth":      "http-basic-auth.json",
	// Steps
	"TerraformDataSourceStep": "terraform-datasource-step.json",
	"HTTPGetStep":             "http-get-step.json",
	"StaticStep":              "static-step.json",
	"ExecStep":                "exec-step.json",
	// Output
	"OutputSpec":         "output-spec.json",
	"ArchiveSpec":        "archive-spec.json",
	"EncodingSpec":       "encoding-spec.json",
	"JSONEncodingSpec":   "json-encoding-spec.json",
	"SinkSpec":           "sink-spec.json",
	"S3SinkSpec":         "s3-sink-spec.json",
	"S3Credentials":      "s3-credentials.json",
	"FilesystemSinkSpec": "filesystem-sink-spec.json",
	"StdoutSinkSpec":     "stdout-sink-spec.json",
}

func main() {
	// Find project root by looking for go.mod
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding project root: %v\n", err)
		os.Exit(1)
	}

	outputDir := filepath.Join(root, "website", "src", "data", "schemas")

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Load package using go/packages
	cfg := &packages.Config{
		Mode: packages.NeedSyntax | packages.NeedFiles | packages.NeedName,
		Dir:  root,
	}

	pkgs, err := packages.Load(cfg, "./apis/v1")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading package: %v\n", err)
		os.Exit(1)
	}

	if len(pkgs) == 0 {
		fmt.Fprintf(os.Stderr, "No packages found\n")
		os.Exit(1)
	}

	// Check for package errors
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				fmt.Fprintf(os.Stderr, "Package error: %v\n", e)
			}
			os.Exit(1)
		}
	}

	// Collect all type specs across files
	typeSpecs := make(map[string]*typeInfo)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			collectTypeSpecs(file, typeSpecs)
		}
	}

	// Generate JSON for each target struct
	for structName, outputFile := range targetStructs {
		info, ok := typeSpecs[structName]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: struct %s not found\n", structName)
			continue
		}

		schema := extractSchema(info)
		outputPath := filepath.Join(outputDir, outputFile)

		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling %s: %v\n", structName, err)
			continue
		}

		if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
			continue
		}

		fmt.Printf("Generated %s\n", outputPath)
	}
}

type typeInfo struct {
	name       string
	doc        string
	structType *ast.StructType
}

func collectTypeSpecs(file *ast.File, typeSpecs map[string]*typeInfo) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			// Get doc comment (prefer GenDecl.Doc for single type, TypeSpec.Doc otherwise)
			var doc string
			if genDecl.Doc != nil && len(genDecl.Specs) == 1 {
				doc = cleanDocComment(genDecl.Doc.Text())
			} else if typeSpec.Doc != nil {
				doc = cleanDocComment(typeSpec.Doc.Text())
			}

			typeSpecs[typeSpec.Name.Name] = &typeInfo{
				name:       typeSpec.Name.Name,
				doc:        doc,
				structType: structType,
			}
		}
	}
}

func extractSchema(info *typeInfo) Schema {
	schema := Schema{
		Name:        info.name,
		Description: info.doc,
		Fields:      []Field{},
	}

	for _, field := range info.structType.Fields.List {
		if len(field.Names) == 0 {
			continue // embedded field
		}

		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			continue
		}

		f := Field{
			Name: fieldName,
		}

		// Parse struct tag
		if field.Tag != nil {
			tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
			f.YAMLKey = parseYAMLKey(tag)
			f.Required, f.Enum = parseValidateTag(tag)
			f.Template = parseTemplateTag(tag)
		}

		// Parse type
		f.Type, f.Ref = parseFieldType(field.Type)

		// Parse doc comment
		f.Description, f.Default = parseFieldDoc(field)

		schema.Fields = append(schema.Fields, f)
	}

	return schema
}

func parseYAMLKey(tag reflect.StructTag) string {
	yamlTag := tag.Get("yaml")
	if yamlTag == "" {
		return ""
	}
	parts := strings.Split(yamlTag, ",")
	return parts[0]
}

func parseValidateTag(tag reflect.StructTag) (required bool, enum []string) {
	validateTag := tag.Get("validate")
	if validateTag == "" {
		return false, nil
	}

	parts := strings.Split(validateTag, ",")
	for _, part := range parts {
		if part == "required" {
			required = true
		}
		if strings.HasPrefix(part, "oneof=") {
			values := strings.TrimPrefix(part, "oneof=")
			enum = strings.Split(values, " ")
		}
	}
	return required, enum
}

func parseTemplateTag(tag reflect.StructTag) bool {
	_, ok := tag.Lookup("template")
	return ok
}

func parseFieldType(expr ast.Expr) (typeName string, ref *string) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, nil
	case *ast.StarExpr:
		inner, innerRef := parseFieldType(t.X)
		if innerRef != nil {
			return inner, innerRef
		}
		// Check if it's a pointer to a known struct type
		if ident, ok := t.X.(*ast.Ident); ok {
			if isKnownStruct(ident.Name) {
				return inner, &ident.Name
			}
		}
		return inner, nil
	case *ast.ArrayType:
		inner, _ := parseFieldType(t.Elt)
		return "[]" + inner, nil
	case *ast.MapType:
		key, _ := parseFieldType(t.Key)
		val, _ := parseFieldType(t.Value)
		return fmt.Sprintf("map[%s]%s", key, val), nil
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name, nil
		}
		return t.Sel.Name, nil
	case *ast.InterfaceType:
		return "any", nil
	default:
		return "unknown", nil
	}
}

func isKnownStruct(name string) bool {
	_, ok := targetStructs[name]
	return ok
}

// Matches patterns like: Default: "gzip", Defaults to "value", Default is "json"
var defaultRegex = regexp.MustCompile(`[Dd]efaults?(?: is|:| to)[:\s]+["']([^"']+)["']|[Dd]efaults?(?: is|:| to)[:\s]+(\$\w+)`)

func parseFieldDoc(field *ast.Field) (description string, defaultVal *string) {
	var docText string

	// Prefer doc comment above field
	if field.Doc != nil {
		docText = field.Doc.Text()
	} else if field.Comment != nil {
		// Fall back to inline comment
		docText = field.Comment.Text()
	}

	if docText == "" {
		return "", nil
	}

	// Clean up the doc comment
	description = cleanDocComment(docText)

	// Extract default value from patterns like: Default: "gzip", Defaults to "value"
	if matches := defaultRegex.FindStringSubmatch(docText); len(matches) > 1 {
		// Check which capture group matched (quoted value or $VAR)
		val := matches[1]
		if val == "" {
			val = matches[2]
		}
		if val != "" {
			val = strings.TrimSpace(val)
			defaultVal = &val
		}
	}

	return description, defaultVal
}

func cleanDocComment(s string) string {
	// Remove leading/trailing whitespace but preserve newlines
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
