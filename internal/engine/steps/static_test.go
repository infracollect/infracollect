package steps

import (
	"path/filepath"
	"testing"

	"github.com/samber/lo"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemMapFs(t *testing.T, files map[string]string) afero.Fs {
	t.Helper()
	fs := afero.NewMemMapFs()
	for path, content := range files {
		dir := filepath.Dir(path)
		if dir != "" {
			require.NoError(t, fs.MkdirAll(dir, 0755))
		}

		require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0644))
	}
	return fs
}

func TestNewStaticStepWithFs(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		filepath    string
		parseAs     *string
		wantData    any
		wantMeta    map[string]string
		wantErr     bool
		errContains string
	}{
		{
			name:     "reads file with relative path",
			files:    map[string]string{"test.txt": "hello world"},
			filepath: "test.txt",
			wantData: map[string]any{"test.txt": "hello world"},
			wantMeta: map[string]string{"filepath": "test.txt"},
		},
		{
			name:     "reads nested file",
			files:    map[string]string{"a/b/c/nested.txt": "nested content"},
			filepath: "a/b/c/nested.txt",
			wantData: map[string]any{"nested.txt": "nested content"},
			wantMeta: map[string]string{"filepath": "a/b/c/nested.txt"},
		},
		{
			name:        "returns error for non-existent file",
			files:       nil,
			filepath:    "nonexistent.txt",
			wantErr:     true,
			errContains: "failed to read filepath",
		},
		{
			name:     "auto-parses JSON file",
			files:    map[string]string{"data.json": `{"name": "test", "count": 10}`},
			filepath: "data.json",
			wantData: map[string]any{"name": "test", "count": float64(10)},
			wantMeta: map[string]string{"filepath": "data.json"},
		},
		{
			name:     "skips JSON parsing when parseAs is raw",
			files:    map[string]string{"data.json": `{"name": "test"}`},
			filepath: "data.json",
			parseAs:  lo.ToPtr("raw"),
			wantData: map[string]any{"data.json": `{"name": "test"}`},
			wantMeta: map[string]string{"filepath": "data.json"},
		},
		{
			name:        "returns error for invalid JSON file",
			files:       map[string]string{"invalid.json": `{not valid json}`},
			filepath:    "invalid.json",
			wantErr:     true,
			errContains: "failed to parse as json",
		},
		{
			name:     "reads empty file",
			files:    map[string]string{"empty.txt": ""},
			filepath: "empty.txt",
			wantData: map[string]any{"empty.txt": ""},
			wantMeta: map[string]string{"filepath": "empty.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newMemMapFs(t, tt.files)
			cfg := StaticStepConfig{Filepath: &tt.filepath, ParseAs: tt.parseAs}

			step := newStaticFileStep("test", fs, cfg)

			result, err := step.Resolve(t.Context())
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, result.Data)
			assert.Equal(t, tt.wantMeta, result.Meta)
		})
	}
}

func TestNewStaticStepWithFs_PathTraversal(t *testing.T) {
	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("allowed", 0755))
	require.NoError(t, afero.WriteFile(baseFs, "secret.txt", []byte("secret"), 0644))
	require.NoError(t, afero.WriteFile(baseFs, "allowed/safe.txt", []byte("safe"), 0644))

	sandboxedFs := afero.NewBasePathFs(baseFs, "allowed")

	tests := []struct {
		name     string
		filepath string
		wantData map[string]any
		wantMeta map[string]string
		wantErr  bool
	}{
		{
			name:     "reads file within sandbox",
			filepath: "safe.txt",
			wantData: map[string]any{"safe.txt": "safe"},
			wantMeta: map[string]string{"filepath": "safe.txt"},
		},
		{
			name:     "blocks path traversal with ../",
			filepath: "../secret.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := newStaticFileStep("test", sandboxedFs, StaticStepConfig{Filepath: &tt.filepath})

			result, err := step.Resolve(t.Context())
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, result.Data)
			assert.Equal(t, tt.wantMeta, result.Meta)
		})
	}
}

func TestNewStaticStep_Validation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         StaticStepConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "error when both filepath and value set",
			cfg: StaticStepConfig{
				Filepath: lo.ToPtr("test.txt"),
				Value:    lo.ToPtr("test value"),
			},
			wantErr:     true,
			errContains: "both filepath and value are set",
		},
		{
			name:        "error when neither filepath nor value set",
			cfg:         StaticStepConfig{},
			wantErr:     true,
			errContains: "neither filepath nor value are set",
		},
		{
			name:    "accepts value only",
			cfg:     StaticStepConfig{Value: lo.ToPtr("test")},
			wantErr: false,
		},
		{
			name:    "accepts filepath only",
			cfg:     StaticStepConfig{Filepath: lo.ToPtr("test.txt")},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStaticStep("test", tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewStaticStep_ValueResolution(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		parseAs     *string
		wantData    any
		wantErr     bool
		errContains string
	}{
		{
			name:     "resolves plain value",
			value:    "test value",
			wantData: map[string]any{"value": "test value"},
		},
		{
			name:     "parses value as JSON when specified",
			value:    `{"key": "value", "number": 42}`,
			parseAs:  lo.ToPtr("json"),
			wantData: map[string]any{"key": "value", "number": float64(42)},
		},
		{
			name:        "returns error for invalid JSON value",
			value:       `{invalid json}`,
			parseAs:     lo.ToPtr("json"),
			wantErr:     true,
			errContains: "failed to parse as json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := newStaticValueStep("test", tt.value, tt.parseAs)

			result, err := step.Resolve(t.Context())
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.ErrorContains(t, err, tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, result.Data)
		})
	}
}
