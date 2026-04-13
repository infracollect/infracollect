package runner_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/steps"
	"github.com/infracollect/infracollect/internal/enginetest"
	"github.com/infracollect/infracollect/internal/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// realRegistry returns a registry with real static and exec step factories
// (no terraform/http — those require external dependencies).
func realRegistry(t *testing.T) *engine.Registry {
	t.Helper()
	reg := engine.NewRegistry(zaptest.NewLogger(t))
	reg.RegisterDependency(engine.AllowedEnvVarsDepKey, []string{})
	require.NoError(t, steps.Register(reg))
	return reg
}

func runE2E(t *testing.T, src []byte, filename string, reg *engine.Registry) (map[string]engine.Result, error) {
	t.Helper()
	tmpl, diags := runner.ParseJobTemplate(src, filename)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

	r, diags := runner.New(zaptest.NewLogger(t), tmpl, reg, nil)
	require.False(t, diags.HasErrors(), "new: %s", diags.Error())

	var (
		out map[string]engine.Result
		err error
	)
	enginetest.SilenceStdout(t, func() {
		out, err = r.Run(t.Context())
	})
	return out, err
}

func TestE2E_StaticValueStep(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "static" "greeting" {
  value = "hello world"
}
`)

	out, err := runE2E(t, src, "static.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "static/greeting")
	data, ok := out["static/greeting"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "hello world", data["value"])
}

func TestE2E_StaticJSONValue(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "static" "config" {
  value    = "{\"db\": \"postgres\", \"port\": 5432}"
  parse_as = "json"
}
`)

	out, err := runE2E(t, src, "json.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "static/config")
	data, ok := out["static/config"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "postgres", data["db"])
	assert.Equal(t, float64(5432), data["port"])
}

func TestE2E_StaticFileStep(t *testing.T) {
	reg := realRegistry(t)

	// The static step uses a BasePathFs rooted at cwd, so the fixture
	// must live inside cwd. Write a temp file and clean it up after.
	const fixture = "testdata_e2e_static.json"
	jsonContent := `{"servers": ["a", "b"]}`
	require.NoError(t, os.WriteFile(fixture, []byte(jsonContent), 0o644))
	t.Cleanup(func() { _ = os.Remove(fixture) })

	src := []byte(fmt.Sprintf(`
step "static" "from_file" {
  filepath = %q
}
`, fixture))

	out, err := runE2E(t, src, "file.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "static/from_file")
	data, ok := out["static/from_file"].Data.(map[string]any)
	require.True(t, ok)
	servers, ok := data["servers"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"a", "b"}, servers)
}

func TestE2E_ExecStep(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "exec" "echo" {
  program = ["echo", "{\"status\": \"ok\"}"]
  format  = "json"
}
`)

	out, err := runE2E(t, src, "exec.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "exec/echo")
	data, ok := out["exec/echo"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", data["status"])
}

func TestE2E_CrossStepReference(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "static" "greeting" {
  value = "hello"
}

step "static" "derived" {
  value = step.static.greeting.data.value
}
`)

	out, err := runE2E(t, src, "chain.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "static/derived")
	data, ok := out["static/derived"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "hello", data["value"])
}

func TestE2E_ForEachWithStaticSteps(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "static" "fan" {
  for_each = { dev = "Development", prod = "Production" }
  value    = each.value
}
`)

	out, err := runE2E(t, src, "foreach.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "static/fan")
	fan, ok := out["static/fan"].Data.(map[string]engine.Result)
	require.True(t, ok)
	require.Len(t, fan, 2)

	devData, ok := fan["dev"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Development", devData["value"])

	prodData, ok := fan["prod"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Production", prodData["value"])
}

func TestE2E_OutputToFilesystem(t *testing.T) {
	reg := realRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "static" "data" {
  value = "test-output"
}

output {
  encoding "json" {}
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runE2E(t, src, "output.hcl", reg)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "static", "data.json"))
	require.NoError(t, err, "expected output file to exist")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "test-output", decoded["value"])
}

func TestE2E_ExecWithInput(t *testing.T) {
	reg := realRegistry(t)

	src := []byte(`
step "static" "config" {
  value = "injected"
}

step "exec" "consume" {
  program = ["cat"]
  format  = "json"

  input {
    greeting = step.static.config.data.value
  }
}
`)

	out, err := runE2E(t, src, "exec-input.hcl", reg)
	require.NoError(t, err)

	require.Contains(t, out, "exec/consume")
	data, ok := out["exec/consume"].Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "injected", data["greeting"])
}

func TestE2E_MultipleStepsWithOutputFilter(t *testing.T) {
	reg := realRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "static" "keep" {
  value = "kept"
}

step "static" "drop" {
  value = "dropped"
}

output {
  steps = [step.static.keep]
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runE2E(t, src, "filter.hcl", reg)
	require.NoError(t, err)

	_, err = os.ReadFile(filepath.Join(dir, "static", "keep.json"))
	assert.NoError(t, err, "kept step should be written")

	_, err = os.ReadFile(filepath.Join(dir, "static", "drop.json"))
	assert.ErrorIs(t, err, os.ErrNotExist, "dropped step should not be written")
}

func TestE2E_ExecStepMeta(t *testing.T) {
	reg := realRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "exec" "meta" {
  program = ["echo", "{\"ok\": true}"]
  format  = "json"
}

output {
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runE2E(t, src, "meta.hcl", reg)
	require.NoError(t, err)

	// The exec step produces meta (exec_program, exec_format, etc.)
	metaBytes, err := os.ReadFile(filepath.Join(dir, "exec", "meta.meta.json"))
	require.NoError(t, err, "expected meta file to be written")

	var meta map[string]string
	require.NoError(t, json.Unmarshal(metaBytes, &meta))
	assert.Contains(t, meta, "exec_program")
	assert.Contains(t, meta, "exec_format")
}
