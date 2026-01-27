package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/samber/do/v2"
)

// CollectorFactory creates a collector from a spec.
// The injector is used to resolve dependencies (e.g., tfclient.Client).
type CollectorFactory func(ctx context.Context, i do.Injector, spec any) (Collector, error)

// StepFactory creates a step from a spec.
// The collector parameter is the resolved collector for steps that require one (nil for standalone steps like static).
type StepFactory func(ctx context.Context, i do.Injector, collector Collector, spec any) (Step, error)

// Registry holds factories for creating collectors and steps by kind.
// It is decoupled from actual dependencies - those are resolved via the injector.
type Registry struct {
	mu         sync.RWMutex
	collectors map[string]CollectorFactory
	steps      map[string]StepFactory
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]CollectorFactory),
		steps:      make(map[string]StepFactory),
	}
}

// RegisterCollector registers a collector factory for the given kind.
func (r *Registry) RegisterCollector(kind string, factory CollectorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[kind] = factory
}

// RegisterStep registers a step factory for the given kind.
func (r *Registry) RegisterStep(kind string, factory StepFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps[kind] = factory
}

// CreateCollector creates a collector of the given kind using the registered factory.
func (r *Registry) CreateCollector(ctx context.Context, i do.Injector, kind string, spec any) (Collector, error) {
	r.mu.RLock()
	factory, ok := r.collectors[kind]
	r.mu.RUnlock()

	if !ok {
		return nil, &UnsupportedTypeError{
			Category:  "collector",
			Kind:      kind,
			Available: r.CollectorKinds(),
		}
	}

	return factory(ctx, i, spec)
}

// CreateStep creates a step of the given kind using the registered factory.
func (r *Registry) CreateStep(ctx context.Context, i do.Injector, kind string, collector Collector, spec any) (Step, error) {
	r.mu.RLock()
	factory, ok := r.steps[kind]
	r.mu.RUnlock()

	if !ok {
		return nil, &UnsupportedTypeError{
			Category:  "step",
			Kind:      kind,
			Available: r.StepKinds(),
		}
	}

	return factory(ctx, i, collector, spec)
}

// CollectorKinds returns all registered collector kinds.
func (r *Registry) CollectorKinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kinds := make([]string, 0, len(r.collectors))
	for kind := range r.collectors {
		kinds = append(kinds, kind)
	}
	return kinds
}

// StepKinds returns all registered step kinds.
func (r *Registry) StepKinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kinds := make([]string, 0, len(r.steps))
	for kind := range r.steps {
		kinds = append(kinds, kind)
	}
	return kinds
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
