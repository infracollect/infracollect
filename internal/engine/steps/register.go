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
	registry.RegisterStep(
		ExecStepKind,
		engine.NewStepFactoryWithoutCollector(ExecStepKind, newExecStep),
	)
}

func newStaticStep(_ context.Context, _ *zap.Logger, id string, spec v1.StaticStep) (engine.Step, error) {
	return NewStaticStep(id, StaticStepConfig{
		Filepath: spec.Filepath,
		Value:    spec.Value,
		ParseAs:  spec.ParseAs,
	})
}

func newExecStep(_ context.Context, logger *zap.Logger, id string, spec v1.ExecStep) (engine.Step, error) {
	return NewExecStep(id, logger, ExecStepConfig{
		Program:    spec.Program,
		Input:      spec.Input,
		WorkingDir: spec.WorkingDir,
		Timeout:    spec.Timeout,
		Format:     spec.Format,
		Env:        spec.Env,
	})
}
