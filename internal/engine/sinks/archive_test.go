package sinks

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"

	"github.com/adrien-f/infracollect/internal/engine/archivers"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSink records all writes for verification.
type mockSink struct {
	writes map[string][]byte
	closed bool
}

func newMockSink() *mockSink {
	return &mockSink{writes: make(map[string][]byte)}
}

func (m *mockSink) Name() string { return "mock" }
func (m *mockSink) Kind() string { return "mock" }

func (m *mockSink) Write(_ context.Context, path string, data io.Reader) error {
	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	m.writes[path] = content
	return nil
}

func (m *mockSink) Close(_ context.Context) error {
	m.closed = true
	return nil
}

// readGzipTarToMap decompresses gzip'd tar data and returns a map of filename -> content.
func readGzipTarToMap(data []byte) (map[string]string, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer lo.Must0(gr.Close())
	tr := tar.NewReader(gr)
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

func newArchiveSinkWithGzip(t *testing.T, archiveName string) (*ArchiveSink, *mockSink) {
	t.Helper()
	archiver, err := archivers.NewTarArchiver("gzip")
	require.NoError(t, err)
	mock := newMockSink()
	return NewArchiveSink(mock, archiver, archiveName), mock
}

func TestArchiveSink_SingleFile(t *testing.T) {
	sink, mockInner := newArchiveSinkWithGzip(t, "output.tar.gz")
	ctx := t.Context()

	err := sink.Write(ctx, "test.json", bytes.NewReader([]byte(`{"key":"value"}`)))
	require.NoError(t, err)

	err = sink.Close(ctx)
	require.NoError(t, err)

	assert.Len(t, mockInner.writes, 1)
	require.Contains(t, mockInner.writes, "output.tar.gz")
	found, err := readGzipTarToMap(mockInner.writes["output.tar.gz"])
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, `{"key":"value"}`, found["test.json"])
	assert.True(t, mockInner.closed, "inner sink should be closed")
}

func TestArchiveSink_MultipleFiles(t *testing.T) {
	sink, mockInner := newArchiveSinkWithGzip(t, "bundle.tar.gz")
	ctx := t.Context()

	files := map[string]string{
		"step1.json": `{"step":1}`,
		"step2.json": `{"step":2}`,
		"step3.json": `{"step":3}`,
	}
	for name, content := range files {
		err := sink.Write(ctx, name, bytes.NewReader([]byte(content)))
		require.NoError(t, err)
	}

	require.NoError(t, sink.Close(ctx))

	assert.Len(t, mockInner.writes, 1)
	require.Contains(t, mockInner.writes, "bundle.tar.gz")
	found, err := readGzipTarToMap(mockInner.writes["bundle.tar.gz"])
	require.NoError(t, err)
	assert.Len(t, found, len(files))
	for name, content := range files {
		assert.Equal(t, content, found[name], "file %s", name)
	}
}

func TestArchiveSink_NameAndKind(t *testing.T) {
	sink, _ := newArchiveSinkWithGzip(t, "output.tar.gz")
	assert.Equal(t, "archive(output.tar.gz)->mock", sink.Name())
	assert.Equal(t, "archive", sink.Kind())
}
