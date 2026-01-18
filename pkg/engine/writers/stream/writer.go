// Package stream provides a writer that outputs to any io.Writer (stdout, stderr, etc.).
package stream

import (
	"context"
	"io"

	"github.com/adrien-f/infracollect/pkg/engine"
)

// Writer implements engine.Writer for streaming output to an io.Writer.
type Writer struct {
	w io.Writer
}

// New creates a new stream writer that writes to the given io.Writer.
func New(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write encodes all results to the underlying io.Writer.
func (w *Writer) Write(ctx context.Context, results map[string]engine.Result, encoder engine.Encoder) error {
	return encoder.Encode(ctx, w.w, results)
}

// Close is a no-op for stream writers since we don't own the underlying writer.
func (w *Writer) Close(ctx context.Context) error {
	return nil
}
