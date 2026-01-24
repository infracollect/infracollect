package engine

import (
	"context"
	"io"
)

// Archiver collects files into an archive format.
type Archiver interface {
	// AddFile adds a file to the archive with the given filename and data.
	AddFile(ctx context.Context, filename string, data io.Reader) error

	// Close finalizes the archive and returns a reader for the complete archive data.
	Close() (io.Reader, error)

	// Extension returns the file extension for this archive type (e.g., ".tar.gz").
	Extension() string
}
