package archivers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readTarEntries decompresses the reader (gzip, zstd, or none) and returns a map of filename -> content.
func readTarEntries(r io.Reader, compression string) (map[string]string, error) {
	var decompressed io.Reader
	switch compression {
	case "gzip":
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer lo.Must0(gr.Close())
		decompressed = gr
	case "zstd":
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		decompressed = zr
	case "none":
		decompressed = r
	default:
		return nil, fmt.Errorf("unknown compression: %s", compression)
	}
	tr := tar.NewReader(decompressed)
	found := make(map[string]string)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		found[h.Name] = string(content)
	}
	return found, nil
}

func TestNewTarArchiver(t *testing.T) {
	tests := []struct {
		name        string
		compression string
		wantExt     string
		wantErr     bool
	}{
		{
			name:        "gzip compression",
			compression: "gzip",
			wantExt:     ".tar.gz",
		},
		{
			name:        "zstd compression",
			compression: "zstd",
			wantExt:     ".tar.zst",
		},
		{
			name:        "no compression",
			compression: "none",
			wantExt:     ".tar",
		},
		{
			name:        "empty defaults to gzip",
			compression: "",
			wantExt:     ".tar.gz",
		},
		{
			name:        "unsupported compression",
			compression: "bzip2",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiver, err := NewTarArchiver(tt.compression)
			if tt.wantErr {
				require.Error(t, err, "NewTarArchiver() expected error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantExt, archiver.Extension())
		})
	}
}

func TestTarArchiver_AddFile(t *testing.T) {
	archiver, err := NewTarArchiver("gzip")
	require.NoError(t, err)

	content := "hello, world!"
	err = archiver.AddFile(t.Context(), "test.txt", bytes.NewReader([]byte(content)))
	require.NoError(t, err)

	reader, err := archiver.Close()
	require.NoError(t, err)

	found, err := readTarEntries(reader, "gzip")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, content, found["test.txt"])
}

func TestTarArchiver_MultipleFiles(t *testing.T) {
	archiver, err := NewTarArchiver("gzip")
	require.NoError(t, err)

	files := map[string]string{
		"file1.txt":     "content1",
		"file2.txt":     "content2",
		"dir/file3.txt": "content3",
	}
	for name, content := range files {
		err = archiver.AddFile(t.Context(), name, bytes.NewReader([]byte(content)))
		require.NoError(t, err)
	}

	reader, err := archiver.Close()
	require.NoError(t, err)

	found, err := readTarEntries(reader, "gzip")
	require.NoError(t, err)
	assert.Len(t, found, len(files))
	for name, content := range files {
		assert.Equal(t, content, found[name], "file %s", name)
	}
}

func TestTarArchiver_Zstd(t *testing.T) {
	archiver, err := NewTarArchiver("zstd")
	require.NoError(t, err)

	content := "zstd compressed content"
	err = archiver.AddFile(t.Context(), "zstd-test.txt", bytes.NewReader([]byte(content)))
	require.NoError(t, err)

	reader, err := archiver.Close()
	require.NoError(t, err)

	found, err := readTarEntries(reader, "zstd")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, content, found["zstd-test.txt"])
}

func TestTarArchiver_NoCompression(t *testing.T) {
	archiver, err := NewTarArchiver("none")
	require.NoError(t, err)

	content := "uncompressed content"
	err = archiver.AddFile(t.Context(), "plain.txt", bytes.NewReader([]byte(content)))
	require.NoError(t, err)

	reader, err := archiver.Close()
	require.NoError(t, err)

	found, err := readTarEntries(reader, "none")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, content, found["plain.txt"])
}

func TestTarArchiver_CloseTwice(t *testing.T) {
	archiver, err := NewTarArchiver("gzip")
	require.NoError(t, err)

	_, err = archiver.Close()
	require.NoError(t, err)

	// Second close should error
	_, err = archiver.Close()
	require.Error(t, err, "Close() second call should error")
}

func TestTarArchiver_AddFileAfterClose(t *testing.T) {
	archiver, err := NewTarArchiver("gzip")
	require.NoError(t, err)

	_, err = archiver.Close()
	require.NoError(t, err)

	ctx := t.Context()
	err = archiver.AddFile(ctx, "test.txt", bytes.NewReader([]byte("content")))
	require.Error(t, err, "AddFile() after Close() should error")
}
