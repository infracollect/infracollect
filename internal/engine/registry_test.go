package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Mock types for testing

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
	Value string
}

type testStepSpec struct {
	Value string
}

type wrongSpec struct{}

func TestNewCollectorFactory(t *testing.T) {
	logger := zap.NewNop()
	ctx := t.Context()

	t.Run("correct spec type returns collector", func(t *testing.T) {
		expectedCollector := &mockCollector{name: "test", kind: "test_kind"}

		factory := NewCollectorFactory("test_kind", func(_ context.Context, _ *zap.Logger, spec testCollectorSpec) (Collector, error) {
			assert.Equal(t, "test_value", spec.Value)
			return expectedCollector, nil
		})

		collector, err := factory(ctx, logger, testCollectorSpec{Value: "test_value"})

		require.NoError(t, err)
		assert.Equal(t, expectedCollector, collector)
	})

	t.Run("wrong spec type returns error", func(t *testing.T) {
		factory := NewCollectorFactory("test_kind", func(_ context.Context, _ *zap.Logger, spec testCollectorSpec) (Collector, error) {
			t.Fatal("factory should not be called with wrong spec type")
			return nil, nil
		})

		collector, err := factory(ctx, logger, wrongSpec{})

		require.Error(t, err)
		assert.Nil(t, collector)
		assert.ErrorContains(t, err, "test_kind")
		assert.ErrorContains(t, err, "wrongSpec")
	})
}

func TestNewStepFactory(t *testing.T) {
	logger := zap.NewNop()
	ctx := t.Context()

	t.Run("correct collector and spec types returns step", func(t *testing.T) {
		expectedStep := &mockStep{name: "test", kind: "test_kind"}
		inputCollector := &mockCollector{name: "collector", kind: "collector_kind"}

		factory := NewStepFactory("test_kind", func(_ context.Context, _ *zap.Logger, id string, c *mockCollector, spec testStepSpec) (Step, error) {
			assert.Equal(t, "step_id", id)
			assert.Equal(t, inputCollector, c)
			assert.Equal(t, "test_value", spec.Value)
			return expectedStep, nil
		})

		step, err := factory(ctx, logger, "step_id", inputCollector, testStepSpec{Value: "test_value"})

		require.NoError(t, err)
		assert.Equal(t, expectedStep, step)
	})

	t.Run("nil collector returns error", func(t *testing.T) {
		factory := NewStepFactory("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ *mockCollector, _ testStepSpec) (Step, error) {
			t.Fatal("factory should not be called with nil collector")
			return nil, nil
		})

		step, err := factory(ctx, logger, "step_id", nil, testStepSpec{})

		require.Error(t, err)
		assert.Nil(t, step)
		assert.ErrorContains(t, err, "test_kind")
		assert.ErrorContains(t, err, "requires a collector")
	})

	t.Run("wrong collector type returns error", func(t *testing.T) {
		type otherCollector struct{ mockCollector }
		wrongCollector := &otherCollector{}

		factory := NewStepFactory("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ *mockCollector, _ testStepSpec) (Step, error) {
			t.Fatal("factory should not be called with wrong collector type")
			return nil, nil
		})

		step, err := factory(ctx, logger, "step_id", wrongCollector, testStepSpec{})

		require.Error(t, err)
		assert.Nil(t, step)
		assert.ErrorContains(t, err, "test_kind")
		assert.ErrorContains(t, err, "step_id")
		assert.ErrorContains(t, err, "otherCollector")
	})

	t.Run("wrong spec type returns error", func(t *testing.T) {
		inputCollector := &mockCollector{name: "collector", kind: "collector_kind"}

		factory := NewStepFactory("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ *mockCollector, _ testStepSpec) (Step, error) {
			t.Fatal("factory should not be called with wrong spec type")
			return nil, nil
		})

		step, err := factory(ctx, logger, "step_id", inputCollector, wrongSpec{})

		require.Error(t, err)
		assert.Nil(t, step)
		assert.ErrorContains(t, err, "test_kind")
		assert.ErrorContains(t, err, "step_id")
		assert.ErrorContains(t, err, "wrongSpec")
	})
}

func TestNewStepFactoryWithoutCollector(t *testing.T) {
	logger := zap.NewNop()
	ctx := t.Context()

	t.Run("correct spec type returns step", func(t *testing.T) {
		expectedStep := &mockStep{name: "test", kind: "test_kind"}

		factory := NewStepFactoryWithoutCollector("test_kind", func(_ context.Context, _ *zap.Logger, id string, spec testStepSpec) (Step, error) {
			assert.Equal(t, "step_id", id)
			assert.Equal(t, "test_value", spec.Value)
			return expectedStep, nil
		})

		step, err := factory(ctx, logger, "step_id", nil, testStepSpec{Value: "test_value"})

		require.NoError(t, err)
		assert.Equal(t, expectedStep, step)
	})

	t.Run("wrong spec type returns error", func(t *testing.T) {
		factory := NewStepFactoryWithoutCollector("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ testStepSpec) (Step, error) {
			t.Fatal("factory should not be called with wrong spec type")
			return nil, nil
		})

		step, err := factory(ctx, logger, "step_id", nil, wrongSpec{})

		require.Error(t, err)
		assert.Nil(t, step)
		assert.ErrorContains(t, err, "test_kind")
		assert.ErrorContains(t, err, "step_id")
		assert.ErrorContains(t, err, "wrongSpec")
	})

	t.Run("nil collector is ignored", func(t *testing.T) {
		expectedStep := &mockStep{name: "test", kind: "test_kind"}

		factory := NewStepFactoryWithoutCollector("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ testStepSpec) (Step, error) {
			return expectedStep, nil
		})

		step, err := factory(ctx, logger, "step_id", nil, testStepSpec{})

		require.NoError(t, err)
		assert.Equal(t, expectedStep, step)
	})

	t.Run("any collector is ignored", func(t *testing.T) {
		expectedStep := &mockStep{name: "test", kind: "test_kind"}
		someCollector := &mockCollector{name: "ignored", kind: "ignored"}

		factory := NewStepFactoryWithoutCollector("test_kind", func(_ context.Context, _ *zap.Logger, _ string, _ testStepSpec) (Step, error) {
			return expectedStep, nil
		})

		step, err := factory(ctx, logger, "step_id", someCollector, testStepSpec{})

		require.NoError(t, err)
		assert.Equal(t, expectedStep, step)
	})
}
