// Package enginetest provides reusable test helpers for the runner and
// pipeline layers. It exports stub collectors, a pre-wired registry, and
// utility functions that tests across the codebase can share.
//
// This package depends only on the engine package — it does NOT import
// runner, so runner's own (package-internal) tests can use it without
// creating an import cycle.
//
// Integration-specific mocks (mock terraform provider, httptest servers)
// stay in their own packages.
package enginetest

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// StubCollector is a no-op Collector that tracks Start/Close calls so tests
// can assert the runner's lifecycle wiring.
type StubCollector struct {
	name        string
	kind        string
	StartErr    error
	CloseErr    error
	Started     bool
	Closed      bool
	CloseCalls  int
	CloseCtxErr error // ctx.Err() observed inside Close
}

func NewStubCollector(name, kind string) *StubCollector {
	return &StubCollector{name: name, kind: kind}
}

func (c *StubCollector) Name() string                { return c.name }
func (c *StubCollector) Kind() string                { return c.kind }
func (c *StubCollector) Start(context.Context) error { c.Started = true; return c.StartErr }
func (c *StubCollector) Close(ctx context.Context) error {
	c.Closed = true
	c.CloseCalls++
	c.CloseCtxErr = ctx.Err()
	return c.CloseErr
}

// StubRegistry wires up a registry where every step's Data is the map of its
// own HCL attributes (evaluated against ctx). Collectors are created lazily
// and can be inspected via Collectors after Run.
type StubRegistry struct {
	Reg        *engine.Registry
	Collectors map[string]*StubCollector
}

// NewStubRegistry creates a registry populated with:
//   - "stub" collector (succeeds)
//   - "stub_failing" collector (Start returns "boom")
//   - "stub_step" step (requires "stub" collector, captures body attrs + collector name)
//   - "stub_nocoll" step (collector-less, captures body attrs)
func NewStubRegistry(t *testing.T) *StubRegistry {
	t.Helper()
	r := &StubRegistry{
		Reg:        engine.NewRegistry(zaptest.NewLogger(t)),
		Collectors: make(map[string]*StubCollector),
	}

	collectorFactory := func(name string, startErr error) engine.CollectorFactory {
		return func(_ *engine.RegistryHelper, _ hcl.Body, _ *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
			c := NewStubCollector(name, "stub")
			c.StartErr = startErr
			r.Collectors[name] = c
			return c, nil
		}
	}
	require.NoError(t, r.Reg.RegisterCollector("stub", collectorFactory("stub", nil)))
	require.NoError(t, r.Reg.RegisterCollector("stub_failing", collectorFactory("stub_failing", errors.New("boom"))))

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
	require.NoError(t, r.Reg.RegisterStep(engine.StepDescriptor{
		Kind:                  "stub_step",
		Factory:               stepFactory,
		RequiresCollector:     true,
		AllowedCollectorKinds: []string{"stub"},
	}))

	noCollFactory := func(_ *engine.RegistryHelper, id string, _ engine.Collector, body hcl.Body, ctx *hcl.EvalContext) (engine.Step, hcl.Diagnostics) {
		data, diags := engine.BodyToMap(body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}
		return engine.StepFunction(id, "stub_nocoll", func(context.Context) (engine.Result, error) {
			return engine.Result{ID: id, Data: data}, nil
		}), nil
	}
	require.NoError(t, r.Reg.RegisterStep(engine.StepDescriptor{
		Kind:    "stub_nocoll",
		Factory: noCollFactory,
	}))

	return r
}

// PipelineRegistry returns a registry populated with no-op factories for
// the collector/step kinds used by pipeline-level tests. BuildPipeline
// consults the descriptors for the known-kinds gate and the collector-binding
// rules, so each step's RequiresCollector / AllowedCollectorKinds must match
// the real integration's contract.
//
// Use this when you only need to test BuildPipeline (DAG shape, diagnostics)
// without running the pipeline.
func PipelineRegistry(t *testing.T) *engine.Registry {
	reg := engine.NewRegistry(zaptest.NewLogger(t))
	stubCollector := engine.CollectorFactory(func(*engine.RegistryHelper, hcl.Body, *hcl.EvalContext) (engine.Collector, hcl.Diagnostics) {
		return nil, nil
	})
	stubStep := engine.StepFactory(func(*engine.RegistryHelper, string, engine.Collector, hcl.Body, *hcl.EvalContext) (engine.Step, hcl.Diagnostics) {
		return nil, nil
	})
	for _, k := range []string{"terraform", "http"} {
		if err := reg.RegisterCollector(k, stubCollector); err != nil {
			panic(err)
		}
	}
	if err := reg.RegisterSteps(
		engine.StepDescriptor{
			Kind:                  "terraform_datasource",
			Factory:               stubStep,
			RequiresCollector:     true,
			AllowedCollectorKinds: []string{"terraform"},
		},
		engine.StepDescriptor{
			Kind:                  "http_get",
			Factory:               stubStep,
			RequiresCollector:     true,
			AllowedCollectorKinds: []string{"http"},
		},
		engine.StepDescriptor{Kind: "static", Factory: stubStep},
		engine.StepDescriptor{Kind: "exec", Factory: stubStep},
	); err != nil {
		panic(err)
	}
	return reg
}

// SilenceStdout redirects os.Stdout to /dev/null for the duration of fn
// and restores it afterward.
func SilenceStdout(t *testing.T, fn func()) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(done)
	}()

	defer func() {
		_ = w.Close()
		<-done
		os.Stdout = orig
	}()

	fn()
}
