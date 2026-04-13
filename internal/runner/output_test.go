package runner

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRunner_Output_ExplicitStdout(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "only" {
  greeting = "hello"
}

output {
  encoding "json" {}
  sink "stdout" {}
}
`)

	out, err := runSilently(t, newRunner(t, src, "stdout.hcl", stub.reg))
	require.NoError(t, err)
	require.Contains(t, out, "stub_nocoll/only")
}

func TestRunner_Output_FilesystemSink(t *testing.T) {
	stub := newStubRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "stub_nocoll" "only" {
  greeting = "hello"
}

output {
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runSilently(t, newRunner(t, src, "fs.hcl", stub.reg))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "stub_nocoll", "only.json"))
	require.NoError(t, err, "expected filesystem sink to write per-step file")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "hello", decoded["greeting"])
}

func TestRunner_Output_TarArchiveToFilesystem(t *testing.T) {
	stub := newStubRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
job {
  name = "archive-job"
}

step "stub_nocoll" "only" {
  greeting = "hello"
}

output {
  encoding "json" {}
  archive "tar" {
    compression = "none"
  }
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runSilently(t, newRunner(t, src, "tar.hcl", stub.reg))
	require.NoError(t, err)

	archivePath := filepath.Join(dir, "archive-job.tar")
	archiveBytes, err := os.ReadFile(archivePath)
	require.NoError(t, err, "expected archive file to exist at %s", archivePath)

	entries := tarEntries(t, archiveBytes)
	require.Contains(t, entries, "stub_nocoll/only.json")

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(entries["stub_nocoll/only.json"], &decoded))
	assert.Equal(t, "hello", decoded["greeting"])
}

func TestRunner_Output_Errors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "unknown sink kind",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  sink "carrier_pigeon" {}
}`,
			wantMsg: "unknown sink kind",
		},
		{
			name: "unknown encoding kind",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  encoding "yaml" {}
  sink "stdout" {}
}`,
			wantMsg: "unknown encoding kind",
		},
		{
			name: "unknown archive kind",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  archive "zip" {}
  sink "stdout" {}
}`,
			wantMsg: "unknown archive kind",
		},
		{
			name: "output without sink",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  encoding "json" {}
}`,
			wantMsg: "output block requires a sink",
		},
		{
			name: "filesystem sink missing path",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  sink "filesystem" {}
}`,
			wantMsg: `failed to decode sink "filesystem"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStubRegistry(t)
			_, err := runSilently(t, newRunner(t, []byte(tc.src), "err.hcl", stub.reg))
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantMsg)
		})
	}
}

// --- output steps filter tests -----------------------------------------------

func TestRunner_Output_StepsFilter(t *testing.T) {
	stub := newStubRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "stub_nocoll" "alpha" {
  greeting = "hello"
}

step "stub_nocoll" "beta" {
  greeting = "world"
}

output {
  steps = [step.stub_nocoll.alpha]
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runSilently(t, newRunner(t, src, "filter.hcl", stub.reg))
	require.NoError(t, err)

	// alpha should be written
	data, err := os.ReadFile(filepath.Join(dir, "stub_nocoll", "alpha.json"))
	require.NoError(t, err, "expected filtered step to be written")
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "hello", decoded["greeting"])

	// beta should NOT be written
	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "beta.json"))
	assert.ErrorIs(t, err, os.ErrNotExist, "excluded step should not be written")
}

func TestRunner_Output_StepsFilterMultiple(t *testing.T) {
	stub := newStubRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "stub_nocoll" "a" {
  v = "1"
}

step "stub_nocoll" "b" {
  v = "2"
}

step "stub_nocoll" "c" {
  v = "3"
}

output {
  steps = [step.stub_nocoll.a, step.stub_nocoll.c]
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runSilently(t, newRunner(t, src, "multi.hcl", stub.reg))
	require.NoError(t, err)

	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "a.json"))
	assert.NoError(t, err, "step a should be written")
	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "b.json"))
	assert.ErrorIs(t, err, os.ErrNotExist, "step b should not be written")
	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "c.json"))
	assert.NoError(t, err, "step c should be written")
}

func TestRunner_Output_NoStepsMeansAll(t *testing.T) {
	stub := newStubRegistry(t)
	dir := t.TempDir()

	src := []byte(fmt.Sprintf(`
step "stub_nocoll" "a" {
  v = "1"
}

step "stub_nocoll" "b" {
  v = "2"
}

output {
  sink "filesystem" {
    path = %q
  }
}
`, dir))

	_, err := runSilently(t, newRunner(t, src, "all.hcl", stub.reg))
	require.NoError(t, err)

	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "a.json"))
	assert.NoError(t, err, "step a should be written")
	_, err = os.ReadFile(filepath.Join(dir, "stub_nocoll", "b.json"))
	assert.NoError(t, err, "step b should be written")
}

func TestRunner_Output_StepsFilterErrors(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantMsg string
	}{
		{
			name: "empty steps list",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  steps = []
  sink "stdout" {}
}`,
			wantMsg: "Empty output steps",
		},
		{
			name: "unknown step reference",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  steps = [step.stub_nocoll.nonexistent]
  sink "stdout" {}
}`,
			wantMsg: "step.stub_nocoll.nonexistent is not declared",
		},
		{
			name: "string instead of reference",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  steps = ["stub_nocoll/only"]
  sink "stdout" {}
}`,
			wantMsg: "Invalid step reference",
		},
		{
			name: "collector reference instead of step",
			src: `
collector "stub" "c" {}

step "stub_step" "only" {
  collector = collector.stub.c
  greeting  = "hi"
}

output {
  steps = [collector.stub.c]
  sink "stdout" {}
}`,
			wantMsg: "Must start with the `step` namespace",
		},
		{
			name: "step reference with extra segments",
			src: `
step "stub_nocoll" "only" {
  greeting = "hi"
}

output {
  steps = [step.stub_nocoll.only.data]
  sink "stdout" {}
}`,
			wantMsg: "Must be exactly `step.<type>.<id>`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStubRegistry(t)

			tmpl, diags := ParseJobTemplate([]byte(tc.src), "err.hcl")
			require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

			_, diags = New(zap.NewNop(), tmpl, stub.reg, nil)
			require.True(t, diags.HasErrors(), "expected pipeline build error")
			assert.Contains(t, diags.Error(), tc.wantMsg)
		})
	}
}

// --- buildOutputPipeline unit tests -----------------------------------------

func TestBuildOutputPipeline_DefaultsWhenNil(t *testing.T) {
	baseCtx := &hcl.EvalContext{}
	enc, sink, err := buildOutputPipeline(t.Context(), nil, baseCtx, "job")
	require.NoError(t, err)
	require.NotNil(t, enc)
	require.NotNil(t, sink)
	assert.Equal(t, "json", enc.FileExtension())
	assert.Equal(t, "stream", sink.Kind())
}

func TestBuildOutputPipeline_ArchiveWrapsInnerSink(t *testing.T) {
	// Parse a tiny template so we get real hcl.Body values on each block.
	tmpl, diags := ParseJobTemplate([]byte(`
output {
  encoding "json" {}
  archive "tar" {
    compression = "gzip"
  }
  sink "stdout" {}
}
`), "wrap.hcl")
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

	_, sink, err := buildOutputPipeline(t.Context(), tmpl.Output, &hcl.EvalContext{}, "job")
	require.NoError(t, err)
	assert.Equal(t, "archive", sink.Kind(), "archive block should wrap the inner sink")
}

// tarEntries reads a plain (uncompressed) tar archive and returns its entries
// as a filename -> bytes map. Used by the tar-archive runner test.
func tarEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(data))
	entries := make(map[string][]byte)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, tr)
		require.NoError(t, err)
		entries[hdr.Name] = buf.Bytes()
	}
	return entries
}
