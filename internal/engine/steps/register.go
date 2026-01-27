package steps

import (
	"context"
	"fmt"

	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/samber/do/v2"
)

const (
	StaticStepKind = "static"
)

// Register registers the built-in step factories with the registry.
func Register(r *engine.Registry) {
	r.RegisterStep(StaticStepKind, staticStepFactory)
}

func staticStepFactory(ctx context.Context, i do.Injector, collector engine.Collector, spec any) (engine.Step, error) {
	staticSpec, ok := spec.(*v1.StaticStep)
	if !ok {
		return nil, fmt.Errorf("expected *v1.StaticStep, got %T", spec)
	}

	cfg := StaticStepConfig{
		ParseAs: staticSpec.ParseAs,
	}

	if staticSpec.Filepath != nil {
		cfg.Filepath = staticSpec.Filepath
	}

	if staticSpec.Value != nil {
		cfg.Value = staticSpec.Value
	}

	// The step ID is set by the pipeline, so we use a placeholder here.
	// The actual ID will be set when adding to the pipeline.
	return NewStaticStep("static", cfg)
}
