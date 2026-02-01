package terraform

import (
	"fmt"

	"github.com/go-logr/zapr"
	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	tfclient "github.com/infracollect/tf-data-client"
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

func newCollector(helper *engine.RegistryHelper, spec v1.TerraformCollector) (engine.Collector, error) {
	client, err := tfclient.New(tfclient.WithLogger(zapr.NewLogger(helper.Logger())))
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform client: %w", err)
	}

	return NewCollector(client, Config{
		Provider: spec.Provider,
		Version:  spec.Version,
		Args:     spec.Args,
	})
}

func newDataSourceStep(_ *engine.RegistryHelper, _ string, collector *Collector, spec v1.TerraformDataSourceStep) (engine.Step, error) {
	return NewDataSourceStep(collector, spec.Name, spec.Args), nil
}
