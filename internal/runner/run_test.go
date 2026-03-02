package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
	"go.uber.org/zap"
)

// stubCollector is a no-op Collector that tracks Start/Close calls so tests
// can assert the runner's lifecycle wiring.
type stubCollector struct {
	name          string
	startErr      error
	closeErr      error
	started       bool
	closed        bool
	closeCalls    int
	closeCtxErr   error // ctx.Err() observed inside Close
	closeCtxAlive bool  // true when ctx was not canceled at the time of Close
}

func (c *stubCollector) Name() string                { return c.name }
func (c *stubCollector) Kind() string                { return "stub" }
func (c *stubCollector) Start(context.Context) error { c.started = true; return c.startErr }
func (c *stubCollector) Close(ctx context.Context) error {
	c.closed = true
	c.closeCalls++
	c.closeCtxErr = ctx.Err()
	c.closeCtxAlive = ctx.Err() == nil
	return c.closeErr
}

// stubRegistry wires up a registry where every step's Data is the map of its
// own HCL attributes (evaluated against ctx). Collectors are created lazily
// and can be inspected via collectors[<id>] after Run.
type stubRegistry struct {
	reg        *engine.Registry
	collectors map[string]*stubCollector
}

func newStubRegistry(t *testing.T) *stubRegistry {
	t.Helper()
	r := &stubRegistry{
		reg:        engine.NewRegistry(zap.NewNop()),
		collectors: make(map[string]*stubCollector),
	}

	collectorFactory := func(name string, startErr error) engine.CollectorFactory {
		return func(_ *engine.RegistryHelper, _ hcl.Body, _ *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
			c := &stubCollector{name: name, startErr: startErr}
			r.collectors[name] = c
			return c, nil
		}
	}
	if err := r.reg.RegisterCollector("stub", collectorFactory("stub", nil)); err != nil {
		t.Fatalf("register stub collector: %v", err)
	}
	if err := r.reg.RegisterCollector("stub_failing", collectorFactory("stub_failing", errors.New("boom"))); err != nil {
		t.Fatalf("register stub_failing collector: %v", err)
	}

	// Step factory that evaluates body attributes through ctx and returns
	// them as the step's Data. Records the bound collector name (if any)
	// under a "__collector" key so tests can assert resolveStepCollector.
	stepFactory := func(_ *engine.RegistryHelper, id string, collector engine.Collector, body hcl.Body, ctx *hcl.EvalContext) (engine.Step, hcl.Diagnostics) {
		data, diags := engine.BodyToMap(body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}
		if collector != nil {
			data["__collector"] = collector.Name()
		}
		meta := map[string]string{"kind": "stub_step"}
		return engine.StepFunction(id, "stub_step", func(context.Context) (engine.Result, error) {
			return engine.Result{ID: id, Data: data, Meta: meta}, nil
		}), nil
	}
	if err := r.reg.RegisterStep(engine.StepDescriptor{
		Kind:                  "stub_step",
		Factory:               stepFactory,
		RequiresCollector:     true,
		AllowedCollectorKinds: []string{"stub"},
	}); err != nil {
		t.Fatalf("register stub_step: %v", err)
	}

	// Collector-less variant for plain/for_each paths.
	noCollFactory := func(_ *engine.RegistryHelper, id string, _ engine.Collector, body hcl.Body, ctx *hcl.EvalContext) (engine.Step, hcl.Diagnostics) {
		data, diags := engine.BodyToMap(body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}
		return engine.StepFunction(id, "stub_nocoll", func(context.Context) (engine.Result, error) {
			return engine.Result{ID: id, Data: data}, nil
		}), nil
	}
	if err := r.reg.RegisterStep(engine.StepDescriptor{
		Kind:    "stub_nocoll",
		Factory: noCollFactory,
	}); err != nil {
		t.Fatalf("register stub_nocoll: %v", err)
	}

	return r
}

// silenceStdout redirects os.Stdout to /dev/null for the duration of fn and
// restores it afterward. Used to swallow writeResults' JSON output during
// runner tests — the tests assert on r.raw, not the encoded stream.
func silenceStdout(t *testing.T, fn func()) {
	t.Helper()
	orig := os.Stdout
	null, err := os.Open(os.DevNull)
	require.NoError(t, err)
	// os.Stdout is expected to be a *os.File, but a pipe works too.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(done)
	}()

	defer func() {
		w.Close()
		<-done
		os.Stdout = orig
		null.Close()
	}()

	fn()
}

func newRunner(t *testing.T, src []byte, filename string, reg *engine.Registry) *Runner {
	t.Helper()
	tmpl, diags := ParseJobTemplate(src, filename)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())

	r, diags := New(zap.NewNop(), tmpl, reg, nil)
	require.False(t, diags.HasErrors(), "new: %s", diags.Error())
	return r
}

func runSilently(t *testing.T, r *Runner) (map[string]engine.Result, error) {
	t.Helper()
	var (
		out map[string]engine.Result
		err error
	)
	silenceStdout(t, func() {
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
	stub := newStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "only" {
  greeting = "hello"
}
`)

	out := runOrFail(t, src, "plain.hcl", stub.reg)

	require.Contains(t, out, "stub_nocoll/only")
	data := out["stub_nocoll/only"].Data.(map[string]any)
	assert.Equal(t, "hello", data["greeting"])
}

func TestRunner_CollectorBinding(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "bound"
}
`)

	out := runOrFail(t, src, "bind.hcl", stub.reg)

	require.Contains(t, stub.collectors, "stub")
	assert.True(t, stub.collectors["stub"].started, "collector should be Started")
	assert.True(t, stub.collectors["stub"].closed, "collector should be Closed via defer")

	data := out["stub_step/s"].Data.(map[string]any)
	assert.Equal(t, "bound", data["label"])
	assert.Equal(t, "stub", data["__collector"], "step should see the bound collector")
}

func TestRunner_CrossStepReference(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "first" {
  val = "hello"
}

step "stub_nocoll" "second" {
  got = step.stub_nocoll.first.data.val
}
`)

	out := runOrFail(t, src, "chain.hcl", stub.reg)

	second := out["stub_nocoll/second"].Data.(map[string]any)
	assert.Equal(t, "hello", second["got"])
}

func TestRunner_ForEachMap(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "fan" {
  for_each = { alpha = "one", beta = "two" }
  label    = each.key
  val      = each.value
}
`)

	out := runOrFail(t, src, "fan.hcl", stub.reg)

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
	stub := newStubRegistry(t)

	src := []byte(`
step "stub_nocoll" "fan" {
  for_each = ["a", "b"]
  val      = each.value
}
`)

	_, err := runSilently(t, newRunner(t, src, "fan.hcl", stub.reg))
	require.Error(t, err)
	assert.ErrorContains(t, err, "for_each")
	assert.ErrorContains(t, err, "map, object, or set of strings")
}

func TestRunner_CollectorStartErrorClosesStartedCollectors(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
collector "stub_failing" "bad" {
}
`)

	_, err := runSilently(t, newRunner(t, src, "fail.hcl", stub.reg))
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
	// A collector that failed Start() was never published to r.collectors,
	// so closeCollectors should not call Close on it.
	if c, ok := stub.collectors["stub_failing"]; ok {
		assert.Equal(t, 0, c.closeCalls, "failed-to-start collector must not be closed")
	}
}

func TestRunner_CloseCollectorsUsesIndependentContext(t *testing.T) {
	stub := newStubRegistry(t)

	src := []byte(`
collector "stub" "c" {
}

step "stub_step" "s" {
  collector = collector.stub.c
  label     = "v"
}
`)

	r := newRunner(t, src, "canceled.hcl", stub.reg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // canceled before Run even starts

	silenceStdout(t, func() {
		_, _ = r.Run(ctx)
	})

	c := stub.collectors["stub"]
	require.NotNil(t, c, "collector should have been created")
	assert.Equal(t, 1, c.closeCalls, "Close must run once even when run ctx is canceled")
	assert.NoError(t, c.closeCtxErr, "Close must receive a fresh, uncanceled context")
	assert.True(t, c.closeCtxAlive, "Close must receive a fresh, uncanceled context")
}

func TestRunner_CloseCollectorsBestEffortOnError(t *testing.T) {
	stub := newStubRegistry(t)

	// Register a second collector kind whose Close fails so we can prove the
	// loop keeps going past an error and still reaches other collectors.
	if err := stub.reg.RegisterCollector("stub_close_fails", func(_ *engine.RegistryHelper, _ hcl.Body, _ *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
		c := &stubCollector{name: "stub_close_fails", closeErr: errors.New("close exploded")}
		stub.collectors["stub_close_fails"] = c
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

	r := newRunner(t, src, "best-effort.hcl", stub.reg)

	var runErr error
	silenceStdout(t, func() {
		_, runErr = r.Run(t.Context())
	})
	assert.NoError(t, runErr, "Close errors must not replace the run's return value")

	require.NotNil(t, stub.collectors["stub_close_fails"])
	require.NotNil(t, stub.collectors["stub"])
	assert.Equal(t, 1, stub.collectors["stub_close_fails"].closeCalls)
	assert.Equal(t, 1, stub.collectors["stub"].closeCalls,
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
