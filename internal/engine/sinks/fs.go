package sinks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/adrien-f/infracollect/internal/engine"
	"github.com/spf13/afero"
)

type FilesystemSink struct {
	fs afero.Fs
}

func NewFilesystemSink(fs afero.Fs) engine.Sink {
	return &FilesystemSink{fs: fs}
}

func NewFilesystemSinkFromPath(path string) (engine.Sink, error) {
	cleanPath := filepath.Clean(path)

	// Ensure the base directory exists
	if err := os.MkdirAll(cleanPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", cleanPath, err)
	}

	return NewFilesystemSink(afero.NewBasePathFs(afero.NewOsFs(), cleanPath)), nil
}

func (s *FilesystemSink) Name() string {
	return fmt.Sprintf("filesystem(%s)", s.fs.Name())
}

func (s *FilesystemSink) Kind() string {
	return "filesystem"
}

func (s *FilesystemSink) Write(ctx context.Context, path string, data io.Reader) (err error) {
	// Ensure parent directories exist
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := s.fs.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	f, err := s.fs.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	if _, err = io.Copy(f, data); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

func (s *FilesystemSink) Close(ctx context.Context) error {
	return nil
}
