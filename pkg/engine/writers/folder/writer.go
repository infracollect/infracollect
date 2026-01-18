// Package folder provides a writer that outputs one file per step to a folder.
package folder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrien-f/infracollect/pkg/engine"
)

// Writer implements engine.Writer for folder output with one file per step.
type Writer struct {
	path string
}

// New creates a new folder writer that writes to the given path.
func New(path string) *Writer {
	return &Writer{path: path}
}

// Write encodes each result to a separate file in the folder.
func (w *Writer) Write(ctx context.Context, results map[string]engine.Result, encoder engine.Encoder) error {
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(w.path, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	ext := encoder.FileExtension()

	for stepID, result := range results {
		filename := fmt.Sprintf("%s.%s", stepID, ext)
		filepath := filepath.Join(w.path, filename)

		f, err := os.Create(filepath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filepath, err)
		}

		if err := encoder.EncodeResult(ctx, f, result); err != nil {
			f.Close()
			return fmt.Errorf("failed to encode result for step %s: %w", stepID, err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close file %s: %w", filepath, err)
		}
	}

	return nil
}

// Close is a no-op for folder writers.
func (w *Writer) Close(ctx context.Context) error {
	return nil
}
