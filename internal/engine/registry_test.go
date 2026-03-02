package engine

import (
	"context"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCollector struct {
	name string
	kind string
}

func (m *mockCollector) Name() string                { return m.name }
func (m *mockCollector) Kind() string                { return m.kind }
func (m *mockCollector) Start(context.Context) error { return nil }
func (m *mockCollector) Close(context.Context) error { return nil }

type mockStep struct {
	name string
	kind string
}

func (m *mockStep) Name() string { return m.name }
func (m *mockStep) Kind() string { return m.kind }
func (m *mockStep) Resolve(context.Context) (Result, error) {
	return Result{}, nil
}

type testCollectorSpec struct {
	Value string `hcl:"value"`
}

type testStepSpec struct {
	Value string `hcl:"value"`
}

// parseBody compiles a small HCL snippet and returns its body for test use.
func parseBody(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclsyntax.ParseConfig([]byte(src), "test.hcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), "parse: %s", diags.Error())
	return file.Body
}

func TestNewCollectorFactory(t *testing.T) {
	t.Run("decodes body and calls typed factory", func(t *testing.T) {
		expected := &mockCollector{name: "test", kind: "test_kind"}

		factory := NewCollectorFactory("test_kind", func(_ *RegistryHelper, _ *hcl.EvalContext, spec testCollectorSpec) (Collector, error) {
			assert.Equal(t, "hello", spec.Value)
			return expected, nil
		})

		body := parseBody(t, `value = "hello"`)
		collector, diags := factory(nil, body, nil)

		require.False(t, diags.HasErrors(), "factory: %s", diags.Error())
		assert.Equal(t, expected, collector)
	})

	t.Run("bad decode surfaces diagnostics", func(t *testing.T) {
		factory := NewCollectorFactory("test_kind", func(_ *RegistryHelper, _ *hcl.EvalContext, _ testCollectorSpec) (Collector, error) {
			t.Fatal("typed factory should not be called on decode error")
			return nil, nil
		})

		body := parseBody(t, `unknown = "x"`)
		collector, diags := factory(nil, body, nil)

		require.True(t, diags.HasErrors())
		assert.Nil(t, collector)
	})
}

func TestNewStepFactory(t *testing.T) {
	t.Run("decodes body and calls typed factory", func(t *testing.T) {
		expected := &mockStep{name: "test", kind: "test_kind"}
		inputCollector := &mockCollector{name: "collector", kind: "collector_kind"}

		factory := NewStepFactory("test_kind", func(_ *RegistryHelper, id string, c *mockCollector, _ *hcl.EvalContext, spec testStepSpec) (Step, error) {
			assert.Equal(t, "step_id", id)
			assert.Equal(t, inputCollector, c)
			assert.Equal(t, "hello", spec.Value)
			return expected, nil
		})

		body := parseBody(t, `value = "hello"`)
		step, diags := factory(nil, "step_id", inputCollector, body, nil)

		require.False(t, diags.HasErrors(), "factory: %s", diags.Error())
		assert.Equal(t, expected, step)
	})

	t.Run("nil collector returns diagnostic", func(t *testing.T) {
		factory := NewStepFactory("test_kind", func(_ *RegistryHelper, _ string, _ *mockCollector, _ *hcl.EvalContext, _ testStepSpec) (Step, error) {
			t.Fatal("typed factory should not be called with nil collector")
			return nil, nil
		})

		step, diags := factory(nil, "step_id", nil, parseBody(t, `value = "x"`), nil)

		require.True(t, diags.HasErrors())
		assert.Nil(t, step)
		assert.Contains(t, diags.Error(), "test_kind")
		assert.Contains(t, diags.Error(), "requires a collector")
	})

	t.Run("wrong collector type returns diagnostic", func(t *testing.T) {
		type otherCollector struct{ mockCollector }
		wrongCollector := &otherCollector{}

		factory := NewStepFactory("test_kind", func(_ *RegistryHelper, _ string, _ *mockCollector, _ *hcl.EvalContext, _ testStepSpec) (Step, error) {
			t.Fatal("typed factory should not be called with wrong collector type")
			return nil, nil
		})

		step, diags := factory(nil, "step_id", wrongCollector, parseBody(t, `value = "x"`), nil)

		require.True(t, diags.HasErrors())
		assert.Nil(t, step)
		assert.Contains(t, diags.Error(), "test_kind")
		assert.Contains(t, diags.Error(), "step_id")
	})
}

func TestRegistry_RegisterStep(t *testing.T) {
	noop := StepFactory(func(*RegistryHelper, string, Collector, hcl.Body, *hcl.EvalContext) (Step, hcl.Diagnostics) {
		return nil, nil
	})

	t.Run("rejects missing Kind", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterStep(StepDescriptor{Factory: noop})
		require.Error(t, err)
		assert.ErrorContains(t, err, "missing Kind")
	})

	t.Run("rejects missing Factory", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterStep(StepDescriptor{Kind: "x"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "missing Factory")
	})

	t.Run("rejects RequiresCollector with no AllowedCollectorKinds", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterStep(StepDescriptor{
			Kind:              "x",
			Factory:           noop,
			RequiresCollector: true,
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "RequiresCollector")
		assert.ErrorContains(t, err, "AllowedCollectorKinds")
	})

	t.Run("rejects collector-less descriptor with AllowedCollectorKinds", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterStep(StepDescriptor{
			Kind:                  "x",
			Factory:               noop,
			RequiresCollector:     false,
			AllowedCollectorKinds: []string{"c"},
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "collector-less")
		assert.ErrorContains(t, err, "AllowedCollectorKinds")
	})

	t.Run("rejects duplicate kind", func(t *testing.T) {
		r := NewRegistry(nil)
		desc := StepDescriptor{Kind: "x", Factory: noop}
		require.NoError(t, r.RegisterStep(desc))
		err := r.RegisterStep(desc)
		require.Error(t, err)
		assert.ErrorContains(t, err, "already registered")
	})

	t.Run("stores valid descriptor", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterStep(StepDescriptor{
			Kind:                  "x",
			Factory:               noop,
			RequiresCollector:     true,
			AllowedCollectorKinds: []string{"c"},
		})
		require.NoError(t, err)
		desc, ok := r.StepDescriptor("x")
		require.True(t, ok)
		assert.Equal(t, "x", desc.Kind)
		assert.True(t, desc.RequiresCollector)
		assert.Equal(t, []string{"c"}, desc.AllowedCollectorKinds)
	})
}

func TestRegistry_RegisterCollector(t *testing.T) {
	noop := CollectorFactory(func(*RegistryHelper, hcl.Body, *hcl.EvalContext) (Collector, hcl.Diagnostics) {
		return nil, nil
	})

	t.Run("rejects empty kind", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterCollector("", noop)
		require.Error(t, err)
		assert.ErrorContains(t, err, "kind is empty")
	})

	t.Run("rejects nil factory", func(t *testing.T) {
		r := NewRegistry(nil)
		err := r.RegisterCollector("x", nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "missing factory")
	})

	t.Run("rejects duplicate kind", func(t *testing.T) {
		r := NewRegistry(nil)
		require.NoError(t, r.RegisterCollector("x", noop))
		err := r.RegisterCollector("x", noop)
		require.Error(t, err)
		assert.ErrorContains(t, err, "already registered")
	})
}

func TestNewTypedStepDescriptor(t *testing.T) {
	// The typed helper must produce a descriptor whose AllowedCollectorKinds
	// and runtime type assertion cannot drift: the collector kind passed to
	// the helper must be the single entry, and RequiresCollector must be on.
	desc := NewTypedStepDescriptor("fetch", "my_collector", func(_ *RegistryHelper, _ string, _ *mockCollector, _ *hcl.EvalContext, _ testStepSpec) (Step, error) {
		return &mockStep{name: "fetch", kind: "fetch"}, nil
	})
	assert.Equal(t, "fetch", desc.Kind)
	assert.True(t, desc.RequiresCollector)
	assert.Equal(t, []string{"my_collector"}, desc.AllowedCollectorKinds)
	require.NotNil(t, desc.Factory)
}

func TestNewTypedStepDescriptorWithoutCollector(t *testing.T) {
	desc := NewTypedStepDescriptorWithoutCollector("fetch", func(_ *RegistryHelper, _ string, _ *hcl.EvalContext, _ testStepSpec) (Step, error) {
		return &mockStep{name: "fetch", kind: "fetch"}, nil
	})
	assert.Equal(t, "fetch", desc.Kind)
	assert.False(t, desc.RequiresCollector)
	assert.Empty(t, desc.AllowedCollectorKinds)
	require.NotNil(t, desc.Factory)
}

func TestNewStepFactoryWithoutCollector(t *testing.T) {
	t.Run("decodes body and calls typed factory", func(t *testing.T) {
		expected := &mockStep{name: "test", kind: "test_kind"}

		factory := NewStepFactoryWithoutCollector("test_kind", func(_ *RegistryHelper, id string, _ *hcl.EvalContext, spec testStepSpec) (Step, error) {
			assert.Equal(t, "step_id", id)
			assert.Equal(t, "hello", spec.Value)
			return expected, nil
		})

		step, diags := factory(nil, "step_id", nil, parseBody(t, `value = "hello"`), nil)

		require.False(t, diags.HasErrors(), "factory: %s", diags.Error())
		assert.Equal(t, expected, step)
	})

	t.Run("ignores the supplied collector", func(t *testing.T) {
		expected := &mockStep{name: "test", kind: "test_kind"}
		someCollector := &mockCollector{name: "ignored", kind: "ignored"}

		factory := NewStepFactoryWithoutCollector("test_kind", func(_ *RegistryHelper, _ string, _ *hcl.EvalContext, _ testStepSpec) (Step, error) {
			return expected, nil
		})

		step, diags := factory(nil, "step_id", someCollector, parseBody(t, `value = "x"`), nil)

		require.False(t, diags.HasErrors())
		assert.Equal(t, expected, step)
	})
}
