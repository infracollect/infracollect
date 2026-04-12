//go:build ignore

// gen-docs generates v2 JSON schema documentation from Go structs with
// hcl:"..." tags. It reads the target registry from docs/registry.yaml,
// parses the listed Go packages via AST, and emits JSON files to
// website/src/data/schemas/.
//
// Usage:
//
//	go run scripts/gen-docs.go [-registry docs/registry.yaml] [-out website/src/data/schemas]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/infracollect/infracollect/scripts/gendocs"
)

func main() {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding project root: %v\n", err)
		os.Exit(1)
	}

	registryPath := flag.String("registry", filepath.Join(root, "docs", "registry.yaml"), "path to registry YAML")
	outputDir := flag.String("out", filepath.Join(root, "website", "src", "data", "schemas"), "output directory for JSON schemas")
	flag.Parse()

	reg, err := gendocs.LoadRegistry(*registryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loading %d targets from %s\n", len(reg.Targets), *registryPath)

	loaded, err := gendocs.LoadPackages(root, reg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading packages: %v\n", err)
		os.Exit(1)
	}

	if err := gendocs.EnsureOutputDir(*outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	idx := gendocs.BuildBlockHeaderIndex(reg)

	var warnings []string
	for _, target := range reg.Targets {
		schema, err := gendocs.ExtractSchema(loaded, reg, idx, target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting %s: %v\n", target.ID, err)
			os.Exit(1)
		}

		// Warn on missing field descriptions.
		for _, attr := range schema.Attributes {
			if attr.Description == "" {
				warnings = append(warnings, fmt.Sprintf("  %s: attribute %q has no description", target.ID, attr.Name))
			}
		}

		if err := gendocs.EmitJSON(schema, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", target.ID, err)
			os.Exit(1)
		}

		fmt.Printf("  %-30s → %s.json\n", target.ID, target.ID)
	}

	if len(warnings) > 0 {
		fmt.Printf("\nWarnings (%d fields missing descriptions):\n", len(warnings))
		for _, w := range warnings {
			fmt.Println(w)
		}
	}

	fmt.Printf("\nGenerated %d schemas to %s\n", len(reg.Targets), *outputDir)
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
