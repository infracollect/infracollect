package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/enginetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/zap/zaptest"
)

// newRunner creates a Runner from raw HCL, failing the test on parse/build errors.
func newRunner(t *testing.T, src []byte, filename string, reg *engine.Registry) *Runner {
	t.Helper()
	tmpl, diags := ParseJobTemplate(src, filename)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

	r, diags := New(zaptest.NewLogger(t), tmpl, reg, nil)
	require.False(t, diags.HasErrors(), "new: %s", diags.Error())
	return r
}

func runSilently(t *testing.T, r *Runner) (map[string]engine.Result, error) {
	t.Helper()
	var (
		out map[string]engine.Result
		err error
	)
	enginetest.SilenceStdout(t, func() {
		out, err = r.Run(t.Context())
	})
	return out, err
}

func runOrFail(t *testing.T, src []byte, filename string, reg *engine.Registry) map[string]engine.Result {
	t.Helper()
	out, err := runSilently(t, newRunner(t, src, filename, reg))
	require.NoError(t, err)
	return out
}

func TestRunner_PlainStep(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "only" {
  greeting = "hello"
}
`)

	out := runOrFail(t, src, "plain.hcl", stub.Reg)

	require.Contains(t, out, "stub_nocoll/only")
	data := out["stub_nocoll/only"].Data.(map[string]any)
	assert.Equal(t, "hello", data["greeting"])
}

func TestRunner_CollectorBinding(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "bound"
}
`)

	out := runOrFail(t, src, "bind.hcl", stub.Reg)

	require.Contains(t, stub.Collectors, "stub")
	assert.True(t, stub.Collectors["stub"].Started, "collector should be Started")
	assert.True(t, stub.Collectors["stub"].Closed, "collector should be Closed via defer")

	data := out["stub_step/s"].Data.(map[string]any)
	assert.Equal(t, "bound", data["label"])
	assert.Equal(t, "stub", data["__collector"], "step should see the bound collector")
}

func TestRunner_CrossStepReference(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "first" {
  val = "hello"
}

step "stub_nocoll" "second" {
  got = step.stub_nocoll.first.data.val
}
`)

	out := runOrFail(t, src, "chain.hcl", stub.Reg)

	second := out["stub_nocoll/second"].Data.(map[string]any)
	assert.Equal(t, "hello", second["got"])
}

func TestRunner_ForEachMap(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "fan" {
  for_each = { alpha = "one", beta = "two" }
  label    = each.key
  val      = each.value
}
`)

	out := runOrFail(t, src, "fan.hcl", stub.Reg)

	fan := out["stub_nocoll/fan"].Data.(map[string]engine.Result)
	require.Len(t, fan, 2)

	alpha := fan["alpha"].Data.(map[string]any)
	assert.Equal(t, "alpha", alpha["label"])
	assert.Equal(t, "one", alpha["val"])

	beta := fan["beta"].Data.(map[string]any)
	assert.Equal(t, "beta", beta["label"])
	assert.Equal(t, "two", beta["val"])
}

func TestRunner_ForEachRejectsList(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "fan" {
  for_each = ["a", "b"]
  val      = each.value
}
`)

	_, err := runSilently(t, newRunner(t, src, "fan.hcl", stub.Reg))
	require.Error(t, err)
	assert.ErrorContains(t, err, "for_each")
	assert.ErrorContains(t, err, "map, object, or set of strings")
}

func TestRunner_CollectorStartErrorClosesStartedCollectors(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
collector "stub_failing" "bad" {
}
`)

	_, err := runSilently(t, newRunner(t, src, "fail.hcl", stub.Reg))
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	if c, ok := stub.Collectors["stub_failing"]; ok {
		assert.Equal(t, 0, c.CloseCalls, "failed-to-start collector must not be closed")
	}
}

func TestRunner_CloseCollectorsUsesIndependentContext(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "v"
}
`)

	r := newRunner(t, src, "canceled.hcl", stub.Reg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // canceled before Run even starts

	enginetest.SilenceStdout(t, func() {
		_, _ = r.Run(ctx)
	})

	c := stub.Collectors["stub"]
	require.NotNil(t, c, "collector should have been created")
	assert.Equal(t, 1, c.CloseCalls, "Close must run once even when run ctx is canceled")
	assert.NoError(t, c.CloseCtxErr, "Close must receive a fresh, uncanceled context")
}

func TestRunner_CloseCollectorsBestEffortOnError(t *testing.T) {
	stub := enginetest.NewStubRegistry(t)

	// Register a second collector kind whose Close fails so we can prove the
	// loop keeps going past an error and still reaches other collectors.
	if err := stub.Reg.RegisterCollector("stub_close_fails", func(_ *engine.RegistryHelper, _ hcl.Body, _ *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
		c := enginetest.NewStubCollector("stub_close_fails", "stub_close_fails")
		c.CloseErr = errors.New("close exploded")
		stub.Collectors["stub_close_fails"] = c
		return c, nil
	}); err != nil {
		t.Fatalf("register stub_close_fails: %v", err)
	}

	src := []byte(`
collector "stub_close_fails" "bad" {
}

collector "stub" "good" {
}

step "stub_step" "s" {
  collector = collector.stub.good
  label     = "v"
}
`)

	r := newRunner(t, src, "best-effort.hcl", stub.Reg)

	var runErr error
	enginetest.SilenceStdout(t, func() {
		_, runErr = r.Run(t.Context())
	})
	assert.NoError(t, runErr, "Close errors must not replace the run's return value")

	require.NotNil(t, stub.Collectors["stub_close_fails"])
	require.NotNil(t, stub.Collectors["stub"])
	assert.Equal(t, 1, stub.Collectors["stub_close_fails"].CloseCalls)
	assert.Equal(t, 1, stub.Collectors["stub"].CloseCalls,
		"second collector must still be closed even if the first Close fails")
}

func TestValidateForEachValue(t *testing.T) {
	cases := []struct {
		name    string
		val     cty.Value
		wantErr string
	}{
		{"null", cty.NullVal(cty.DynamicPseudoType), "null"},
		{"map", cty.MapVal(map[string]cty.Value{"k": cty.StringVal("v")}), ""},
		{"object", cty.ObjectVal(map[string]cty.Value{"k": cty.StringVal("v")}), ""},
		{"set-of-strings", cty.SetVal([]cty.Value{cty.StringVal("a")}), ""},
		{"set-of-numbers", cty.SetVal([]cty.Value{cty.NumberIntVal(1)}), "set must contain strings"},
		{"list", cty.ListVal([]cty.Value{cty.StringVal("a")}), "map, object, or set of strings"},
		{"tuple", cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.NumberIntVal(1)}), "map, object, or set of strings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateForEachValue(tc.val)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
