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
	"go.uber.org/zap"
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
			_, err := NewExecStep("test", zap.NewNop(), tt.cfg)
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

func TestExecStep_JSONOutput(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", `echo '{"key": "value", "number": 42}'`},
		Format:  lo.ToPtr("json"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	expected := map[string]any{"key": "value", "number": float64(42)}
	assert.Equal(t, expected, result.Data)
	assert.Equal(t, "json", result.Meta["exec_format"])
}

func TestExecStep_RawOutput(t *testing.T) {
	output := "raw output data"
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", "printf '%s' 'raw output data'"},
		Format:  lo.ToPtr("raw"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	expectedEncoded := base64.StdEncoding.EncodeToString([]byte(output))
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, expectedEncoded, data["output"])
	assert.Equal(t, "raw", result.Meta["exec_format"])
}

func TestExecStep_DefaultFormat(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", `echo '{"default": true}'`},
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	expected := map[string]any{"default": true}
	assert.Equal(t, expected, result.Data)
	assert.Equal(t, "json", result.Meta["exec_format"])
}

func TestExecStep_Input(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
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
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", "echo 'error message' >&2; exit 1"},
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "command failed")
	assert.ErrorContains(t, err, "error message")
}

func TestExecStep_Timeout(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", "sleep 10"},
		Timeout: lo.ToPtr("100ms"),
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "timed out")
}

func TestExecStep_Environment(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
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

func TestExecStep_AllowedEnvFiltering(t *testing.T) {
	// Set up two env vars: one secret and one allowed
	require.NoError(t, os.Setenv("SECRET_VAR", "topsecret"))
	require.NoError(t, os.Setenv("ALLOWED_VAR", "allowed"))
	defer func() {
		_ = os.Unsetenv("SECRET_VAR")
		_ = os.Unsetenv("ALLOWED_VAR")
	}()

	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program:    []string{"sh", "-c", `echo "{\"secret\": \"$SECRET_VAR\", \"allowed\": \"$ALLOWED_VAR\"}"`},
		Format:     lo.ToPtr("json"),
		AllowedEnv: []string{"ALLOWED_VAR"},
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	// SECRET_VAR should be empty (not passed through), ALLOWED_VAR should be present
	assert.Equal(t, "", data["secret"])
	assert.Equal(t, "allowed", data["allowed"])
}

func TestExecStep_AllowedEnvEmptyOnlyPassesSafeVars(t *testing.T) {
	// When AllowedEnv is empty/nil, only safe vars (PATH, HOME, etc.) are passed.
	// This ensures security by default - users must explicitly allow env vars.
	require.NoError(t, os.Setenv("SECRET_VAR", "topsecret"))
	defer func() { _ = os.Unsetenv("SECRET_VAR") }()

	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", `echo "{\"secret\": \"$SECRET_VAR\"}"`},
		Format:  lo.ToPtr("json"),
		// AllowedEnv is nil/unset here - SECRET_VAR should NOT be passed
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	// SECRET_VAR is not in safeEnvVars, so it should be empty
	assert.Equal(t, "", data["secret"])
}

func TestExecStep_WorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program:    []string{"sh", "-c", `echo "{\"file_exists\": \"$(test -f test.txt && echo true || echo false)\"}"`},
		WorkingDir: lo.ToPtr(tmpDir),
		Format:     lo.ToPtr("json"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "true", data["file_exists"])
}

func TestExecStep_RelativeWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	cwd, err := os.Getwd()
	require.NoError(t, err)

	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program:    []string{"sh", "-c", `echo "{\"pwd\": \"$(pwd)\"}"`},
		WorkingDir: lo.ToPtr("."),
		Format:     lo.ToPtr("json"),
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, cwd, data["pwd"])
}

func TestExecStep_InvalidJSONOutput(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", "echo 'not valid json'"},
		Format:  lo.ToPtr("json"),
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse output as JSON")
}

func TestExecStep_Meta(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"sh", "-c", `echo '{"ok": true}'`},
	})
	require.NoError(t, err)

	result, err := step.Resolve(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "sh -c echo '{\"ok\": true}'", result.Meta["exec_program"])
	assert.Equal(t, "json", result.Meta["exec_format"])
}

func TestExecStep_CommandNotFound(t *testing.T) {
	step, err := NewExecStep("test", zap.NewNop(), ExecStepConfig{
		Program: []string{"nonexistent-command-xyz"},
	})
	require.NoError(t, err)

	_, err = step.Resolve(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "command failed")
}
