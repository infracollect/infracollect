package engine_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEncoder encodes Data and Meta as trivial, recognizable readers and can be
// told to fail at either step.
type fakeEncoder struct {
	ext       string
	resultErr error
	metaErr   error
}

func (e fakeEncoder) EncodeResult(_ context.Context, result engine.Result) (io.Reader, error) {
	if e.resultErr != nil {
		return nil, e.resultErr
	}
	return strings.NewReader("data:" + fmt.Sprint(result.Data)), nil
}

func (e fakeEncoder) EncodeMeta(_ context.Context, meta map[string]string) (io.Reader, error) {
	if e.metaErr != nil {
		return nil, e.metaErr
	}
	return strings.NewReader(fmt.Sprintf("meta:%v", meta)), nil
}

func (e fakeEncoder) FileExtension() string { return e.ext }

// recordingSink captures every write keyed by path, in order, and tracks Close.
type recordingSink struct {
	writes   map[string]string
	order    []string
	closed   bool
	writeErr error
	closeErr error
}

func newRecordingSink() *recordingSink {
	return &recordingSink{writes: make(map[string]string)}
}

func (s *recordingSink) Name() string { return "recording" }
func (s *recordingSink) Kind() string { return "recording" }

func (s *recordingSink) Write(_ context.Context, path string, data io.Reader) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	s.writes[path] = string(b)
	s.order = append(s.order, path)
	return nil
}

func (s *recordingSink) Close(_ context.Context) error {
	s.closed = true
	return s.closeErr
}

func TestResultWriter_WritesResultUnderExtension(t *testing.T) {
	sink := newRecordingSink()
	w := engine.NewResultWriter(fakeEncoder{ext: "json"}, sink)

	require.NoError(t, w.Write(t.Context(), "terraform/pods", engine.Result{Data: "x"}))

	assert.Equal(t, "data:x", sink.writes["terraform/pods.json"])
	// No meta present, so no meta file.
	assert.Equal(t, []string{"terraform/pods.json"}, sink.order)
}

func TestResultWriter_WritesMetaWhenPresent(t *testing.T) {
	sink := newRecordingSink()
	w := engine.NewResultWriter(fakeEncoder{ext: "json"}, sink)

	err := w.Write(t.Context(), "http/page", engine.Result{
		Data: "body",
		Meta: map[string]string{"source": "api"},
	})
	require.NoError(t, err)

	assert.Equal(t, "data:body", sink.writes["http/page.json"])
	assert.Contains(t, sink.writes["http/page.meta.json"], "source")
	// Data file is written before its meta file.
	assert.Equal(t, []string{"http/page.json", "http/page.meta.json"}, sink.order)
}

func TestResultWriter_Close_ClosesSink(t *testing.T) {
	sink := newRecordingSink()
	w := engine.NewResultWriter(fakeEncoder{ext: "json"}, sink)

	require.NoError(t, w.Close(t.Context()))
	assert.True(t, sink.closed)
}

func TestResultWriter_Write_EncodeResultError(t *testing.T) {
	sink := newRecordingSink()
	w := engine.NewResultWriter(fakeEncoder{ext: "json", resultErr: errors.New("boom")}, sink)

	err := w.Write(t.Context(), "terraform/pods", engine.Result{Data: "x"})
	assert.ErrorContains(t, err, "failed to encode result terraform/pods")
	assert.Empty(t, sink.order, "nothing should be written when encoding fails")
}

func TestResultWriter_Write_SinkWriteError(t *testing.T) {
	sink := newRecordingSink()
	sink.writeErr = errors.New("disk full")
	w := engine.NewResultWriter(fakeEncoder{ext: "json"}, sink)

	err := w.Write(t.Context(), "terraform/pods", engine.Result{Data: "x"})
	assert.ErrorContains(t, err, "failed to write result terraform/pods")
}

func TestResultWriter_Write_EncodeMetaError(t *testing.T) {
	sink := newRecordingSink()
	w := engine.NewResultWriter(fakeEncoder{ext: "json", metaErr: errors.New("bad meta")}, sink)

	err := w.Write(t.Context(), "http/page", engine.Result{
		Data: "body",
		Meta: map[string]string{"source": "api"},
	})
	assert.ErrorContains(t, err, "failed to encode meta http/page")
	// The data file is written before the meta encode is attempted.
	assert.Equal(t, []string{"http/page.json"}, sink.order)
}
