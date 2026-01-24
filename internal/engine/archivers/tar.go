package archivers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"

	"github.com/adrien-f/infracollect/internal/engine"
	"github.com/klauspost/compress/zstd"
)

// CompressionType defines supported compression algorithms.
type CompressionType string

const (
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
	CompressionNone CompressionType = "none"
)

// TarArchiver creates tar archives with optional compression.
type TarArchiver struct {
	buf         *bytes.Buffer
	compressor  io.WriteCloser
	tarWriter   *tar.Writer
	compression CompressionType
	closed      bool
}

// NewTarArchiver creates a new tar archiver with the specified compression.
// Supported compression types: "gzip", "zstd", "none".
// If compression is empty, defaults to "gzip".
func NewTarArchiver(compression string) (engine.Archiver, error) {
	ct := CompressionType(compression)
	if ct == "" {
		ct = CompressionGzip
	}

	buf := new(bytes.Buffer)
	var compressor io.WriteCloser
	var err error

	switch ct {
	case CompressionGzip:
		compressor = gzip.NewWriter(buf)
	case CompressionZstd:
		compressor, err = zstd.NewWriter(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd writer: %w", err)
		}
	case CompressionNone:
		compressor = &nopWriteCloser{buf}
	default:
		return nil, fmt.Errorf("unsupported compression type: %s", compression)
	}

	tarWriter := tar.NewWriter(compressor)

	return &TarArchiver{
		buf:         buf,
		compressor:  compressor,
		tarWriter:   tarWriter,
		compression: ct,
	}, nil
}

// AddFile adds a file to the tar archive.
func (a *TarArchiver) AddFile(ctx context.Context, filename string, data io.Reader) error {
	if a.closed {
		return fmt.Errorf("archiver is closed")
	}

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled: %w", err)
	}

	// Read all data to determine size
	content, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read file data: %w", err)
	}

	header := &tar.Header{
		Name: filename,
		Mode: 0644,
		Size: int64(len(content)),
	}

	if err := a.tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := a.tarWriter.Write(content); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}

	return nil
}

// Close finalizes the tar archive and returns a reader for the complete archive data.
func (a *TarArchiver) Close() (io.Reader, error) {
	if a.closed {
		return nil, fmt.Errorf("archiver already closed")
	}
	a.closed = true

	// Close tar writer first
	if err := a.tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Close compressor
	if err := a.compressor.Close(); err != nil {
		return nil, fmt.Errorf("failed to close compressor: %w", err)
	}

	return bytes.NewReader(a.buf.Bytes()), nil
}

// Extension returns the file extension for this archive type.
func (a *TarArchiver) Extension() string {
	switch a.compression {
	case CompressionGzip:
		return ".tar.gz"
	case CompressionZstd:
		return ".tar.zst"
	case CompressionNone:
		return ".tar"
	default:
		return ".tar"
	}
}

// nopWriteCloser wraps a Writer to provide a no-op Close method.
type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error {
	return nil
}
