package steps

import (
	"context"

	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"go.uber.org/zap"
)

func Register(registry *engine.Registry) {
	registry.RegisterStep(
		StaticStepKind,
		engine.NewStepFactoryWithoutCollector(StaticStepKind, newStaticStep),
	)
}

func newStaticStep(_ context.Context, _ *zap.Logger, id string, spec v1.StaticStep) (engine.Step, error) {
	return NewStaticStep(id, StaticStepConfig{
		Filepath: spec.Filepath,
		Value:    spec.Value,
		ParseAs:  spec.ParseAs,
	})
}
