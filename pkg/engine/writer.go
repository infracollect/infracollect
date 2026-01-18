package engine

import "context"

// Writer handles output destinations (stdout, folder, zip, etc.).
type Writer interface {
	Closer
	// Write outputs results using the provided encoder.
	// Stream writers encode all results to a single output.
	// Folder/zip writers encode each result to separate files.
	Write(ctx context.Context, results map[string]Result, encoder Encoder) error
}
