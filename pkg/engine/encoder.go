package engine

import (
	"context"
	"io"
)

// Encoder transforms results into a specific format (JSON, YAML, etc.).
type Encoder interface {
	// Encode writes all results to w (used by stream writer).
	Encode(ctx context.Context, w io.Writer, results map[string]Result) error

	// EncodeResult writes a single result's Data to w (used by folder/zip writers).
	EncodeResult(ctx context.Context, w io.Writer, result Result) error

	// FileExtension returns extension without dot (e.g., "json").
	FileExtension() string
}
