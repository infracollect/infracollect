package sinks

import (
	"context"
	"fmt"
	"io"

	"github.com/adrien-f/infracollect/internal/engine"
)

type StreamSink struct {
	w io.Writer
}

func NewStreamSink(w io.Writer) engine.Sink {
	return &StreamSink{w: w}
}

func (s *StreamSink) Name() string {
	return "stream"
}

func (s *StreamSink) Kind() string {
	return "stream"
}

func (s *StreamSink) Write(ctx context.Context, path string, data io.Reader) error {
	if _, err := io.Copy(s.w, data); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}
	return nil
}

func (s *StreamSink) Close(ctx context.Context) error {
	return nil
}
