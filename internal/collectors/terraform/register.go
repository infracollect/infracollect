package terraform

import (
	"context"
	"fmt"

	tfclient "github.com/adrien-f/tf-data-client"
	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/samber/do/v2"
)

// Register registers the terraform collector and step factories with the registry.
// Dependencies (tfclient.Client) are resolved from the injector when factories are called.
func Register(r *engine.Registry) {
	r.RegisterCollector(CollectorKind, collectorFactory)
	r.RegisterStep(DataSourceStepName, dataSourceStepFactory)
}

func collectorFactory(ctx context.Context, i do.Injector, spec any) (engine.Collector, error) {
	tfSpec, ok := spec.(*v1.TerraformCollector)
	if !ok {
		return nil, fmt.Errorf("expected *v1.TerraformCollector, got %T", spec)
	}

	client, err := do.Invoke[Client](i)
	if err != nil {
		return nil, fmt.Errorf("resolving terraform client: %w", err)
	}

	return NewCollector(client, Config{
		Provider: tfSpec.Provider,
		Version:  tfSpec.Version,
		Args:     tfSpec.Args,
	})
}

func dataSourceStepFactory(ctx context.Context, i do.Injector, collector engine.Collector, spec any) (engine.Step, error) {
	dsSpec, ok := spec.(*v1.TerraformDataSourceStep)
	if !ok {
		return nil, fmt.Errorf("expected *v1.TerraformDataSourceStep, got %T", spec)
	}

	tfCollector, ok := collector.(*Collector)
	if !ok {
		return nil, fmt.Errorf("terraform_datasource step requires terraform collector, got %s", collector.Kind())
	}

	return NewDataSourceStep(tfCollector, dsSpec.Name, dsSpec.Args), nil
}

// NewTFClientProvider creates a provider function for the DI container.
// This allows lazy initialization of the terraform client.
func NewTFClientProvider(opts ...tfclient.ClientOption) func(do.Injector) (Client, error) {
	return func(i do.Injector) (Client, error) {
		return tfclient.New(opts...)
	}
}
