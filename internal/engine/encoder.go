package engine

import (
	"context"
	"io"
)

// Encoder transforms results into a specific format (JSON, YAML, etc.).
type Encoder interface {
	// EncodeResult encodes a single result's Data to a reader.
	EncodeResult(ctx context.Context, result Result) (io.Reader, error)

	// FileExtension returns extension without dot (e.g., "json").
	FileExtension() string
}
