package terraform

import (
	"context"
	"fmt"

	tfclient "github.com/adrien-f/tf-data-client"
	"github.com/go-logr/zapr"
	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"go.uber.org/zap"
)

func Register(registry *engine.Registry) {
	registry.RegisterCollector(
		CollectorKind,
		engine.NewCollectorFactory(CollectorKind, newCollector),
	)

	registry.RegisterStep(
		DataSourceStepKind,
		engine.NewStepFactory(DataSourceStepKind, newDataSourceStep),
	)
}

func newCollector(_ context.Context, logger *zap.Logger, spec v1.TerraformCollector) (engine.Collector, error) {
	client, err := tfclient.New(tfclient.WithLogger(zapr.NewLogger(logger)))
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform client: %w", err)
	}

	return NewCollector(client, Config{
		Provider: spec.Provider,
		Version:  spec.Version,
		Args:     spec.Args,
	})
}

func newDataSourceStep(_ context.Context, _ *zap.Logger, _ string, collector *Collector, spec v1.TerraformDataSourceStep) (engine.Step, error) {
	return NewDataSourceStep(collector, spec.Name, spec.Args), nil
}
