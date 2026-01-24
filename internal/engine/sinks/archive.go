package sinks

import (
	"context"
	"fmt"
	"io"

	"github.com/adrien-f/infracollect/internal/engine"
)

// ArchiveSink wraps a sink and collects all writes into an archive.
// On Close, it finalizes the archive and writes a single file to the inner sink.
type ArchiveSink struct {
	inner       engine.Sink
	archiver    engine.Archiver
	archiveName string
}

// NewArchiveSink creates a new archive sink that wraps the given inner sink.
// All writes are collected into the archiver, and on Close, the complete archive
// is written to the inner sink with the specified archive name.
func NewArchiveSink(inner engine.Sink, archiver engine.Archiver, archiveName string) *ArchiveSink {
	return &ArchiveSink{
		inner:       inner,
		archiver:    archiver,
		archiveName: archiveName,
	}
}

// Name returns the name of this sink.
func (s *ArchiveSink) Name() string {
	return fmt.Sprintf("archive(%s)->%s", s.archiveName, s.inner.Name())
}

// Kind returns the kind of this sink.
func (s *ArchiveSink) Kind() string {
	return "archive"
}

// Write adds a file to the archive.
func (s *ArchiveSink) Write(ctx context.Context, path string, data io.Reader) error {
	if err := s.archiver.AddFile(ctx, path, data); err != nil {
		return fmt.Errorf("failed to add file to archive: %w", err)
	}
	return nil
}

// Close finalizes the archive and writes it to the inner sink.
func (s *ArchiveSink) Close(ctx context.Context) error {
	reader, err := s.archiver.Close()
	if err != nil {
		return fmt.Errorf("failed to finalize archive: %w", err)
	}

	if err := s.inner.Write(ctx, s.archiveName, reader); err != nil {
		return fmt.Errorf("failed to write archive to sink: %w", err)
	}

	if err := s.inner.Close(ctx); err != nil {
		return fmt.Errorf("failed to close inner sink: %w", err)
	}

	return nil
}
