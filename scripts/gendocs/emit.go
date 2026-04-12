package gendocs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureOutputDir creates the output directory if it doesn't exist.
func EnsureOutputDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// EmitJSON writes a DocSchema as a formatted JSON file.
func EmitJSON(schema *DocSchema, outputDir string) error {
	filename := schema.ID + ".json"
	path := filepath.Join(outputDir, filename)

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", schema.ID, err)
	}

	// Ensure trailing newline.
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
