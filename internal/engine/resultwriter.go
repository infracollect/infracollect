package engine

import (
	"context"
	"fmt"
)

// ResultWriter encodes results through an Encoder and streams them to a Sink.
// It owns the file-naming convention (<id>.<ext> for data, <id>.meta.<ext> for
// metadata), the rule that metadata is only written when present, and the
// close ordering. Callers feed it results one at a time, in whatever order and
// selection they choose, then Close once.
//
// The Encoder and Sink are the real seams here — ResultWriter is a single
// concrete orchestrator over them, not an interface.
type ResultWriter struct {
	encoder Encoder
	sink    Sink
}

// NewResultWriter pairs an encoder with a sink. An ArchiveSink may be passed as
// the sink; ResultWriter neither knows nor cares whether the sink archives.
func NewResultWriter(encoder Encoder, sink Sink) *ResultWriter {
	return &ResultWriter{encoder: encoder, sink: sink}
}

// Write encodes result, and its metadata when present, and writes them to the
// sink under "<id>.<ext>" and "<id>.meta.<ext>".
func (w *ResultWriter) Write(ctx context.Context, id string, result Result) error {
	ext := w.encoder.FileExtension()

	reader, err := w.encoder.EncodeResult(ctx, result)
	if err != nil {
		return fmt.Errorf("failed to encode result %s: %w", id, err)
	}
	if err := w.sink.Write(ctx, id+"."+ext, reader); err != nil {
		return fmt.Errorf("failed to write result %s: %w", id, err)
	}

	if len(result.Meta) > 0 {
		metaReader, err := w.encoder.EncodeMeta(ctx, result.Meta)
		if err != nil {
			return fmt.Errorf("failed to encode meta %s: %w", id, err)
		}
		if err := w.sink.Write(ctx, id+".meta."+ext, metaReader); err != nil {
			return fmt.Errorf("failed to write meta %s: %w", id, err)
		}
	}
	return nil
}

// Close finalizes the sink, flushing any archive it wraps.
func (w *ResultWriter) Close(ctx context.Context) error {
	return w.sink.Close(ctx)
}
