// Package bundle provides writers for bundled outputs like ZIP archives.
package bundle

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrien-f/infracollect/pkg/engine"
)

// ZipWriter implements engine.Writer for ZIP archive output with one entry per step.
type ZipWriter struct {
	path string
}

// NewZipWriter creates a new ZIP writer that writes to the given path.
func NewZipWriter(path string) *ZipWriter {
	return &ZipWriter{path: path}
}

// Write encodes each result to a separate entry in the ZIP archive.
func (w *ZipWriter) Write(ctx context.Context, results map[string]engine.Result, encoder engine.Encoder) error {
	// Ensure parent directory exists
	dir := filepath.Dir(w.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	f, err := os.Create(w.path)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	ext := encoder.FileExtension()

	for stepID, result := range results {
		filename := fmt.Sprintf("%s.%s", stepID, ext)

		entry, err := zw.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %w", filename, err)
		}

		if err := encoder.EncodeResult(ctx, entry, result); err != nil {
			return fmt.Errorf("failed to encode result for step %s: %w", stepID, err)
		}
	}

	return nil
}

// Close is a no-op for ZIP writers (file is closed in Write).
func (w *ZipWriter) Close(ctx context.Context) error {
	return nil
}
