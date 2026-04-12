// Package gendocs extracts HCL schema documentation from Go source files.
//
// It parses hcl:"..." struct tags via AST (without importing integration
// packages), extracts Go field types and doc comments, and emits v2 JSON
// schema files consumed by the website's PropertyReference component.
//
// The package is used exclusively by scripts/gen-docs.go and is not part
// of the infracollect runtime.
package gendocs
