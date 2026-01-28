package engine

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/samber/lo"
	"go.uber.org/zap"
)

type CollectorFactory func(ctx context.Context, logger *zap.Logger, input any) (Collector, error)
type StepFactory func(ctx context.Context, logger *zap.Logger, id string, collector Collector, input any) (Step, error)

// TypedCollectorFactory is a strongly-typed collector factory.
// T is the concrete spec type (e.g. v1.HTTPCollector).
type TypedCollectorFactory[T any] func(ctx context.Context, logger *zap.Logger, spec T) (Collector, error)

// TypedStepFactory is a strongly-typed step factory.
// C is the concrete collector type (e.g. *http.Collector).
// S is the concrete step spec type (e.g. v1.HTTPGetStep).
type TypedStepFactory[C Collector, S any] func(ctx context.Context, logger *zap.Logger, id string, collector C, spec S) (Step, error)

// TypedStepFactoryWithoutCollector is a strongly-typed step factory for steps that don't require a collector.
// S is the concrete step spec type (e.g. v1.StaticStep).
type TypedStepFactoryWithoutCollector[S any] func(ctx context.Context, logger *zap.Logger, id string, spec S) (Step, error)

// NewCollectorFactory wraps a typed collector factory into a generic CollectorFactory.
// It centralizes the unsafe cast from any → T and provides a clear error if the type mismatches.
func NewCollectorFactory[T any](kind string, f TypedCollectorFactory[T]) CollectorFactory {
	return func(ctx context.Context, logger *zap.Logger, input any) (Collector, error) {
		spec, ok := input.(T)
		if !ok {
			return nil, fmt.Errorf("invalid collector spec for kind %q: %T", kind, input)
		}
		return f(ctx, logger, spec)
	}
}

// NewStepFactory wraps a typed step factory into a generic StepFactory.
// It centralizes the unsafe casts from Collector → C and any → S and provides clear errors.
func NewStepFactory[C Collector, S any](kind string, f TypedStepFactory[C, S]) StepFactory {
	return func(ctx context.Context, logger *zap.Logger, id string, collector Collector, input any) (Step, error) {
		if collector == nil {
			return nil, fmt.Errorf("step kind %q requires a collector, got nil", kind)
		}

		typedCollector, ok := collector.(C)
		if !ok {
			return nil, fmt.Errorf("invalid collector type for step %q with id %s: %T", kind, id, collector)
		}

		spec, ok := input.(S)
		if !ok {
			return nil, fmt.Errorf("invalid step spec for kind %q with id %s: %T", kind, id, input)
		}

		return f(ctx, logger, id, typedCollector, spec)
	}
}

// NewStepFactoryWithoutCollector wraps a typed step factory for steps that don't require a collector.
// It centralizes the unsafe cast from any → S and provides a clear error if the type mismatches.
func NewStepFactoryWithoutCollector[S any](kind string, f TypedStepFactoryWithoutCollector[S]) StepFactory {
	return func(ctx context.Context, logger *zap.Logger, id string, _ Collector, input any) (Step, error) {
		spec, ok := input.(S)
		if !ok {
			return nil, fmt.Errorf("invalid step spec for kind %q with id %s: %T", kind, id, input)
		}

		return f(ctx, logger, id, spec)
	}
}

// UnsupportedTypeError is returned when a collector or step kind is not registered.
type UnsupportedTypeError struct {
	Category  string   // "collector" or "step"
	Kind      string   // the requested kind
	Available []string // registered kinds
}

func (e *UnsupportedTypeError) Error() string {
	if len(e.Available) == 0 {
		return fmt.Sprintf("unsupported %s type %q: no %ss registered", e.Category, e.Kind, e.Category)
	}
	return fmt.Sprintf("unsupported %s type %q (available: %v)", e.Category, e.Kind, e.Available)
}

type Registry struct {
	mu         sync.RWMutex
	collectors map[string]CollectorFactory
	steps      map[string]StepFactory
	logger     *zap.Logger
}

func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		collectors: make(map[string]CollectorFactory),
		steps:      make(map[string]StepFactory),
		logger:     logger,
	}
}

func (r *Registry) RegisterCollector(kind string, factory CollectorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[kind] = factory
}

func (r *Registry) RegisterStep(kind string, factory StepFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps[kind] = factory
}

func (r *Registry) CreateCollector(ctx context.Context, kind string, spec any) (Collector, error) {
	r.mu.RLock()
	factory, ok := r.collectors[kind]
	available := r.availableCollectors()
	r.mu.RUnlock()
	if !ok {
		return nil, &UnsupportedTypeError{Category: "collector", Kind: kind, Available: available}
	}
	return factory(ctx, r.logger, spec)
}

func (r *Registry) CreateStep(ctx context.Context, kind string, id string, collector Collector, spec any) (Step, error) {
	r.mu.RLock()
	factory, ok := r.steps[kind]
	available := r.availableSteps()
	r.mu.RUnlock()
	if !ok {
		return nil, &UnsupportedTypeError{Category: "step", Kind: kind, Available: available}
	}
	return factory(ctx, r.logger, id, collector, spec)
}

func (r *Registry) AvailableCollectors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.availableCollectors()
}

func (r *Registry) availableCollectors() []string {
	collectors := lo.Keys(r.collectors)
	slices.Sort(collectors)
	return collectors
}

func (r *Registry) AvailableSteps() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.availableSteps()
}

func (r *Registry) availableSteps() []string {
	steps := lo.Keys(r.steps)
	slices.Sort(steps)
	return steps
}
