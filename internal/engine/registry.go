package engine

import (
	"fmt"
	"slices"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// CollectorFactory produces a Collector from a raw hcl.Body and an
// hcl.EvalContext. The factory owns the second-pass decode of the body into
// the integration-local typed config struct, so diagnostics surface with
// source ranges intact.
type CollectorFactory func(
	helper *RegistryHelper,
	body hcl.Body,
	ctx *hcl.EvalContext,
) (Collector, hcl.Diagnostics)

// StepFactory produces a Step. `collector` is non-nil for step kinds that
// bound themselves via `collector = collector.<type>.<id>` at parse time.
type StepFactory func(
	helper *RegistryHelper,
	id string,
	collector Collector,
	body hcl.Body,
	ctx *hcl.EvalContext,
) (Step, hcl.Diagnostics)

// StepDescriptor is the public, semantic contract a step integration gives
// to the registry. BuildPipeline consults the descriptor at template-build
// time to validate collector bindings before any runtime factory runs — so
// users see "step X requires kind Y, got Z" diagnostics with source ranges
// instead of late failures inside the typed-factory backstop.
type StepDescriptor struct {
	// Kind is the `step "<kind>" "<id>"` label that selects this entry.
	Kind string

	// Factory is invoked at runtime to decode and construct the step.
	Factory StepFactory

	// RequiresCollector, when true, makes `collector = ...` mandatory on
	// every instance of this step. Omitting the attribute is a build error.
	RequiresCollector bool

	// AllowedCollectorKinds is the set of collector kinds this step can be
	// bound to. Empty means this step is collector-less and must NOT declare
	// a `collector` attribute at all. Registering a descriptor with
	// RequiresCollector=true and an empty AllowedCollectorKinds is a
	// contradiction and is rejected at RegisterStep time.
	//
	// Multi-kind caveat: listing more than one kind only composes safely
	// when every listed kind is implemented by the same concrete Go
	// collector type expected by the factory — see the NOTE on
	// NewStepFactory. Heterogeneous kinds need a hand-rolled StepFactory (or
	// a future NewMultiKindStepDescriptor helper).
	AllowedCollectorKinds []string
}

// TypedCollectorFactory builds a Collector from an already-decoded config
// struct. NewCollectorFactory wraps it with the gohcl decode step.
// The eval context is also passed so the integration can further resolve
// any leftover `,remain` body fields its config carries.
type TypedCollectorFactory[T any] func(
	helper *RegistryHelper,
	ctx *hcl.EvalContext,
	cfg T,
) (Collector, error)

// TypedStepFactory builds a Step with a concrete collector type and a
// decoded step config struct. ctx is also passed for leftover-body resolution.
type TypedStepFactory[C Collector, S any] func(
	helper *RegistryHelper,
	id string,
	collector C,
	ctx *hcl.EvalContext,
	cfg S,
) (Step, error)

// TypedStepFactoryWithoutCollector builds a collector-less step (static, exec).
type TypedStepFactoryWithoutCollector[S any] func(
	helper *RegistryHelper,
	id string,
	ctx *hcl.EvalContext,
	cfg S,
) (Step, error)

const (
	AllowedEnvVarsDepKey = "allowedEnvVars"
)

// NewCollectorFactory wraps a typed factory with a gohcl.DecodeBody pass,
// surfacing decode errors as hcl.Diagnostics and constructor errors as
// non-ranged diagnostics.
func NewCollectorFactory[T any](kind string, f TypedCollectorFactory[T]) CollectorFactory {
	return func(helper *RegistryHelper, body hcl.Body, ctx *hcl.EvalContext) (Collector, hcl.Diagnostics) {
		var cfg T
		if diags := gohcl.DecodeBody(body, ctx, &cfg); diags.HasErrors() {
			return nil, diags
		}
		c, err := f(helper, ctx, cfg)
		if err != nil {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Failed to construct %q collector", kind),
				Detail:   err.Error(),
			}}
		}
		return c, nil
	}
}

// NewStepFactory wraps a typed step factory. Validates that `collector` is
// non-nil and of the expected concrete type, then decodes the body and
// dispatches.
//
// NOTE: NewStepFactory[C, S] assumes every allowed collector kind for the
// step is implemented by the same concrete Go collector type C. That holds
// for single-kind steps and for multi-kind steps backed by one shared
// collector implementation. If a future step needs genuinely heterogeneous
// collector kinds (different Go types behind different kinds), register a
// custom StepFactory — or a future NewMultiKindStepDescriptor helper —
// instead of this generic wrapper. BuildPipeline's AllowedCollectorKinds
// check will still enforce the descriptor's kind list, but the runtime
// type assertion here is the invariant that must line up.
func NewStepFactory[C Collector, S any](kind string, f TypedStepFactory[C, S]) StepFactory {
	return func(helper *RegistryHelper, id string, collector Collector, body hcl.Body, ctx *hcl.EvalContext) (Step, hcl.Diagnostics) {
		if collector == nil {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Step kind %q requires a collector", kind),
				Detail:   fmt.Sprintf("Step %q (%s) did not declare a collector attribute.", id, kind),
			}}
		}
		typedCollector, ok := collector.(C)
		if !ok {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Incompatible collector for step %q", kind),
				Detail: fmt.Sprintf(
					"Step %q (%s) was bound to collector of type %T, which is not compatible with kind %q.",
					id, kind, collector, kind,
				),
			}}
		}
		var cfg S
		if diags := gohcl.DecodeBody(body, ctx, &cfg); diags.HasErrors() {
			return nil, diags
		}
		step, err := f(helper, id, typedCollector, ctx, cfg)
		if err != nil {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Failed to construct %q step %q", kind, id),
				Detail:   err.Error(),
			}}
		}
		return step, nil
	}
}

// NewStepFactoryWithoutCollector is the collector-less variant used by
// static and exec steps.
func NewStepFactoryWithoutCollector[S any](kind string, f TypedStepFactoryWithoutCollector[S]) StepFactory {
	return func(helper *RegistryHelper, id string, _ Collector, body hcl.Body, ctx *hcl.EvalContext) (Step, hcl.Diagnostics) {
		var cfg S
		if diags := gohcl.DecodeBody(body, ctx, &cfg); diags.HasErrors() {
			return nil, diags
		}
		step, err := f(helper, id, ctx, cfg)
		if err != nil {
			return nil, hcl.Diagnostics{&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Failed to construct %q step %q", kind, id),
				Detail:   err.Error(),
			}}
		}
		return step, nil
	}
}

// NewTypedStepDescriptor produces a fully-formed StepDescriptor for a step
// that requires a collector of a specific kind. It binds the typed factory
// (whose generic parameter C is the concrete Collector implementation that
// the step type-asserts against) and the descriptor's AllowedCollectorKinds
// in one call, so the two sources of truth cannot drift apart when an
// integration is added.
//
// For the uncommon multi-kind case (a step accepting several collector
// kinds via a supertype or interface), construct a StepDescriptor by hand.
func NewTypedStepDescriptor[C Collector, S any](
	kind, collectorKind string,
	f TypedStepFactory[C, S],
) StepDescriptor {
	return StepDescriptor{
		Kind:                  kind,
		Factory:               NewStepFactory(kind, f),
		RequiresCollector:     true,
		AllowedCollectorKinds: []string{collectorKind},
	}
}

// NewTypedStepDescriptorWithoutCollector produces a StepDescriptor for a
// collector-less step (static, exec, ...). It is the typed counterpart of
// NewTypedStepDescriptor and exists so integrations never touch raw
// StepDescriptor fields for the common cases.
func NewTypedStepDescriptorWithoutCollector[S any](
	kind string,
	f TypedStepFactoryWithoutCollector[S],
) StepDescriptor {
	return StepDescriptor{
		Kind:    kind,
		Factory: NewStepFactoryWithoutCollector(kind, f),
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

type RegistryHelper struct {
	logger *zap.Logger
	deps   map[string]any
}

func (h *RegistryHelper) Logger() *zap.Logger {
	return h.logger
}

func GetRegistryDependency[T any](h *RegistryHelper, key string) (T, bool) {
	dep, ok := h.deps[key]
	if !ok {
		var zero T
		return zero, false
	}
	typedDep, ok := dep.(T)
	return typedDep, ok
}

func MustGetRegistryDependency[T any](h *RegistryHelper, key string) T {
	dep, ok := GetRegistryDependency[T](h, key)
	if !ok {
		panic(fmt.Sprintf("registry dependency %q of type %T not found", key, dep))
	}
	return dep
}

type Registry struct {
	mu         sync.RWMutex
	collectors map[string]CollectorFactory
	steps      map[string]StepDescriptor
	helper     *RegistryHelper
}

func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		collectors: make(map[string]CollectorFactory),
		steps:      make(map[string]StepDescriptor),
		helper: &RegistryHelper{
			logger: logger,
			deps:   make(map[string]any),
		},
	}
}

func (r *Registry) Helper() *RegistryHelper {
	return r.helper
}

func (r *Registry) RegisterDependency(key string, dep any) {
	r.helper.deps[key] = dep
}

// RegisterCollector installs a collector factory under its kind. It rejects
// an empty kind, a nil factory, or a duplicate registration.
func (r *Registry) RegisterCollector(kind string, factory CollectorFactory) error {
	if kind == "" {
		return fmt.Errorf("collector kind is empty")
	}
	if factory == nil {
		return fmt.Errorf("collector %q is missing factory", kind)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.collectors[kind]; exists {
		return fmt.Errorf("collector kind %q is already registered", kind)
	}
	r.collectors[kind] = factory
	return nil
}

// RegisterStep installs a step descriptor under its Kind. It rejects
// malformed descriptors up-front so mistakes surface at boot time rather
// than as late factory-dispatch failures:
//
//   - a missing Kind or Factory,
//   - RequiresCollector=true with no AllowedCollectorKinds (needs a collector
//     but won't say which kinds are acceptable),
//   - RequiresCollector=false with a non-empty AllowedCollectorKinds (says
//     "these kinds are fine" while also being collector-less),
//   - a duplicate Kind.
func (r *Registry) RegisterStep(desc StepDescriptor) error {
	if desc.Kind == "" {
		return fmt.Errorf("step descriptor is missing Kind")
	}
	if desc.Factory == nil {
		return fmt.Errorf("step descriptor %q is missing Factory", desc.Kind)
	}
	if desc.RequiresCollector && len(desc.AllowedCollectorKinds) == 0 {
		return fmt.Errorf(
			"step descriptor %q sets RequiresCollector=true but declares no AllowedCollectorKinds",
			desc.Kind,
		)
	}
	if !desc.RequiresCollector && len(desc.AllowedCollectorKinds) > 0 {
		return fmt.Errorf(
			"step descriptor %q is collector-less (RequiresCollector=false) but declares AllowedCollectorKinds %v",
			desc.Kind, desc.AllowedCollectorKinds,
		)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.steps[desc.Kind]; exists {
		return fmt.Errorf("step kind %q is already registered", desc.Kind)
	}
	r.steps[desc.Kind] = desc
	return nil
}

// RegisterSteps installs several step descriptors in one call, short-
// circuiting on the first invalid descriptor. It exists to keep integration
// Register() functions flat rather than forcing them to unroll an
// `if err := RegisterStep(...); err != nil { return err }` per step.
func (r *Registry) RegisterSteps(descs ...StepDescriptor) error {
	for _, d := range descs {
		if err := r.RegisterStep(d); err != nil {
			return err
		}
	}
	return nil
}

// StepDescriptor returns the registered descriptor for a step kind, if any.
// BuildPipeline uses this to enforce collector-binding rules at template
// build time rather than at runtime factory dispatch.
func (r *Registry) StepDescriptor(kind string) (StepDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	desc, ok := r.steps[kind]
	return desc, ok
}

// CreateCollector decodes and constructs a collector of the given kind from
// an HCL body evaluated against ctx.
func (r *Registry) CreateCollector(kind string, body hcl.Body, ctx *hcl.EvalContext) (Collector, hcl.Diagnostics) {
	r.mu.RLock()
	factory, ok := r.collectors[kind]
	available := r.availableCollectors()
	r.mu.RUnlock()
	if !ok {
		return nil, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Unsupported collector type %q", kind),
			Detail:   fmt.Sprintf("Available collectors: %v", available),
		}}
	}
	return factory(r.helper, body, ctx)
}

// CreateStep decodes and constructs a step of the given kind.
func (r *Registry) CreateStep(
	kind string,
	id string,
	collector Collector,
	body hcl.Body,
	ctx *hcl.EvalContext,
) (Step, hcl.Diagnostics) {
	r.mu.RLock()
	desc, ok := r.steps[kind]
	available := r.availableSteps()
	r.mu.RUnlock()
	if !ok {
		return nil, hcl.Diagnostics{&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("Unsupported step type %q", kind),
			Detail:   fmt.Sprintf("Available steps: %v", available),
		}}
	}
	return desc.Factory(r.helper, id, collector, body, ctx)
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
