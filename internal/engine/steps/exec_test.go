package steps

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewExecStep_Validation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ExecStepConfig
		wantErr     bool
		errContains string
	}{
		{
			name:        "error when program is empty",
			cfg:         ExecStepConfig{Program: []string{}},
			wantErr:     true,
			errContains: "program is required",
		},
		{
			name:        "error when program is nil",
			cfg:         ExecStepConfig{Program: nil},
			wantErr:     true,
			errContains: "program is required",
		},
		{
			name:        "error when timeout is invalid",
			cfg:         ExecStepConfig{Program: []string{"echo"}, Timeout: lo.ToPtr("invalid")},
			wantErr:     true,
			errContains: "invalid timeout",
		},
		{
			name:    "accepts valid program",
			cfg:     ExecStepConfig{Program: []string{"echo", "hello"}},
			wantErr: false,
		},
		{
			name:    "accepts valid timeout",
			cfg:     ExecStepConfig{Program: []string{"echo"}, Timeout: lo.ToPtr("5s")},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExecStep("test", zaptest.NewLogger(t), tt.cfg)
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

func TestExecStep_OutputFormats(t *testing.T) {
	rawOutput := "raw output data"
	rawEncoded := base64.StdEncoding.EncodeToString([]byte(rawOutput))

	tests := []struct {
		name       string
		cfg        ExecStepConfig
		wantData   any
		wantFormat string
	}{
		{
			name: "json format",
			cfg: ExecStepConfig{
				Program: []string{"sh", "-c", `echo '{"key": "value", "number": 42}'`},
				Format:  lo.ToPtr("json"),
			},
			wantData:   map[string]any{"key": "value", "number": float64(42)},
			wantFormat: "json",
		},
		{
			name: "raw format",
			cfg: ExecStepConfig{
				Program: []string{"sh", "-c", "printf '%s' 'raw output data'"},
				Format:  lo.ToPtr("raw"),
			},
			wantData:   map[string]any{"output": rawEncoded},
			wantFormat: "raw",
		},
		{
			name: "default format is json",
			cfg: ExecStepConfig{
				Program: []string{"sh", "-c", `echo '{"default": true}'`},
			},
			wantData:   map[string]any{"default": true},
			wantFormat: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := NewExecStep("test", zaptest.NewLogger(t), tt.cfg)
			require.NoError(t, err)

			result, err := step.Resolve(t.Context())
			require.NoError(t, err)

			assert.Equal(t, tt.wantData, result.Data)
			assert.Equal(t, tt.wantFormat, result.Meta["exec_format"])
		})
	}
}

func TestExecStep_Input(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", "cat"},
		Input:   map[string]any{"hello": "world", "count": 42},
		Format:  lo.ToPtr("json"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	expected := map[string]any{"hello": "world", "count": float64(42)}
	assert.Equal(t, expected, result.Data)
}

func TestExecStep_NonZeroExit(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", "echo 'error message' >&2; exit 1"},
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "command failed")
	assert.ErrorContains(t, err, "error message")
}

func TestExecStep_Timeout(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", "sleep 10"},
		Timeout: lo.ToPtr("100ms"),
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
}

func TestExecStep_Environment(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", `echo "{\"test_var\": \"$TEST_VAR\", \"home_set\": \"$(test -n \"$HOME\" && echo true || echo false)\"}"`},
		Env:     map[string]string{"TEST_VAR": "custom_value"},
		Format:  lo.ToPtr("json"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "custom_value", data["test_var"])
	assert.Equal(t, "true", data["home_set"])
}

func TestExecStep_EnvFiltering(t *testing.T) {
	require.NoError(t, os.Setenv("SECRET_VAR", "topsecret"))
	require.NoError(t, os.Setenv("ALLOWED_VAR", "allowed"))
	t.Cleanup(func() {
		_ = os.Unsetenv("SECRET_VAR")
		_ = os.Unsetenv("ALLOWED_VAR")
	})

	tests := []struct {
		name       string
		allowedEnv []string
		wantSecret string
		wantAllow  string
	}{
		{
			name:       "explicit allowlist passes only listed vars",
			allowedEnv: []string{"ALLOWED_VAR"},
			wantSecret: "",
			wantAllow:  "allowed",
		},
		{
			name:       "nil allowlist blocks non-safe vars",
			allowedEnv: nil,
			wantSecret: "",
			wantAllow:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
				Program:    []string{"sh", "-c", `echo "{\"secret\": \"$SECRET_VAR\", \"allowed\": \"$ALLOWED_VAR\"}"`},
				Format:     lo.ToPtr("json"),
				AllowedEnv: tt.allowedEnv,
			})
			require.NoError(t, err)

			result, err := step.Resolve(t.Context())
			require.NoError(t, err)

			data, ok := result.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.wantSecret, data["secret"])
			assert.Equal(t, tt.wantAllow, data["allowed"])
		})
	}
}

func TestExecStep_WorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test content"), 0644))

	cwd, err := os.Getwd()
	require.NoError(t, err)

	tests := []struct {
		name    string
		workDir string
		program string
		key     string
		want    string
	}{
		{
			name:    "absolute path",
			workDir: tmpDir,
			program: `echo "{\"result\": \"$(test -f test.txt && echo true || echo false)\"}"`,
			key:     "result",
			want:    "true",
		},
		{
			name:    "relative path resolves to cwd",
			workDir: ".",
			program: `echo "{\"result\": \"$(pwd)\"}"`,
			key:     "result",
			want:    cwd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
				Program:    []string{"sh", "-c", tt.program},
				WorkingDir: lo.ToPtr(tt.workDir),
				Format:     lo.ToPtr("json"),
			})
			require.NoError(t, err)

			result, err := step.Resolve(t.Context())
			require.NoError(t, err)

			data, ok := result.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.want, data[tt.key])
		})
	}
}

func TestExecStep_InvalidJSONOutput(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", "echo 'not valid json'"},
		Format:  lo.ToPtr("json"),
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse output as JSON")
}

func TestExecStep_Meta(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"sh", "-c", `echo '{"ok": true}'`},
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "sh -c echo '{\"ok\": true}'", result.Meta["exec_program"])
	assert.Equal(t, "json", result.Meta["exec_format"])
}

func TestExecStep_CommandNotFound(t *testing.T) {
	step, err := NewExecStep("test", zaptest.NewLogger(t), ExecStepConfig{
		Program: []string{"nonexistent-command-xyz"},
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "command failed")
}
