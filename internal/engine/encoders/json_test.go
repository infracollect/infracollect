package encoders

import (
	"io"
	"testing"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONEncoder_FileExtension(t *testing.T) {
	enc := NewJSONEncoder("  ")
	assert.Equal(t, "json", enc.FileExtension())
}

func TestJSONEncoder_EncodeResult(t *testing.T) {
	tests := []struct {
		name     string
		indent   string
		result   engine.Result
		expected string
	}{
		{
			name:   "simple map with indent",
			indent: "  ",
			result: engine.Result{Data: map[string]any{"key": "value"}},
			expected: `{
  "key": "value"
}
`,
		},
		{
			name:     "simple map no indent",
			indent:   "",
			result:   engine.Result{Data: map[string]any{"key": "value"}},
			expected: "{\"key\":\"value\"}\n",
		},
		{
			name:     "string data",
			indent:   "",
			result:   engine.Result{Data: "hello"},
			expected: "\"hello\"\n",
		},
		{
			name:     "numeric data",
			indent:   "",
			result:   engine.Result{Data: float64(42)},
			expected: "42\n",
		},
		{
			name:     "null data",
			indent:   "",
			result:   engine.Result{Data: nil},
			expected: "null\n",
		},
		{
			name:   "nested map",
			indent: "",
			result: engine.Result{Data: map[string]any{
				"outer": map[string]any{"inner": "deep"},
			}},
			expected: "{\"outer\":{\"inner\":\"deep\"}}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewJSONEncoder(tt.indent)
			reader, err := enc.EncodeResult(t.Context(), tt.result)
			require.NoError(t, err)

			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestJSONEncoder_EncodeMeta(t *testing.T) {
	tests := []struct {
		name     string
		meta     map[string]string
		expected string
	}{
		{
			name:     "single entry",
			meta:     map[string]string{"provider": "aws"},
			expected: "{\"provider\":\"aws\"}\n",
		},
		{
			name:     "empty meta",
			meta:     map[string]string{},
			expected: "{}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewJSONEncoder("")
			reader, err := enc.EncodeMeta(t.Context(), tt.meta)
			require.NoError(t, err)

			data, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))
		})
	}
}

func TestJSONEncoder_EncodeResult_UnmarshalableData(t *testing.T) {
	enc := NewJSONEncoder("")
	// Channels cannot be JSON-encoded.
	_, err := enc.EncodeResult(t.Context(), engine.Result{Data: make(chan int)})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to encode result as JSON")
}
