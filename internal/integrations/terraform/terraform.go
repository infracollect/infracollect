package terraform

import (
	"fmt"

	"github.com/go-logr/zapr"
	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
	tfclient "github.com/infracollect/tf-data-client"
)

// CollectorConfig is the HCL-level shape of a `collector "terraform" "<id>" { ... }` block.
//
//	collector "terraform" "k8s" {
//	  provider    = "hashicorp/kubernetes"
//	  version     = "2.0.0"
//	  config_path = env.KUBECONFIG
//	}
//
// All attributes other than `provider` / `version` are forwarded to the
// provider as its Configure() arguments, matching the behavior of Terraform's
// `provider "kubernetes" { ... }` block.
type CollectorConfig struct {
	Provider string   `hcl:"provider"`
	Version  string   `hcl:"version,optional"`
	Rest     hcl.Body `hcl:",remain"`
}

// DataSourceStepConfig is the HCL-level shape of a
// `step "terraform_datasource" "<id>" { ... }` block.
type DataSourceStepConfig struct {
	DataSource *DataSourceBlock `hcl:"datasource,block"`
}

// DataSourceBlock is the inner `datasource "<kind>" { ... }` block.
// The kind is a label; the remaining body is held unevaluated so the
// integration can resolve provider-specific attributes against the runner's
// eval context.
type DataSourceBlock struct {
	Kind string   `hcl:"kind,label"`
	Body hcl.Body `hcl:",remain"`
}

func Register(registry *engine.Registry) error {
	if err := registry.RegisterCollector(
		CollectorKind,
		engine.NewCollectorFactory(CollectorKind, newCollector),
	); err != nil {
		return err
	}

	return registry.RegisterSteps(
		engine.NewTypedStepDescriptor(DataSourceStepKind, CollectorKind, newDataSourceStep),
	)
}

func newCollector(
	helper *engine.RegistryHelper,
	ctx *hcl.EvalContext,
	cfg CollectorConfig,
) (engine.Collector, error) {
	client, err := tfclient.New(tfclient.WithLogger(zapr.NewLogger(helper.Logger())))
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform client: %w", err)
	}

	args, err := engine.EvalBodyToMap(cfg.Rest, ctx, "terraform collector config")
	if err != nil {
		return nil, err
	}

	return NewCollector(client, Config{
		Provider: cfg.Provider,
		Version:  cfg.Version,
		Args:     args,
	})
}

func newDataSourceStep(
	_ *engine.RegistryHelper,
	_ string,
	collector *Collector,
	ctx *hcl.EvalContext,
	cfg DataSourceStepConfig,
) (engine.Step, error) {
	if cfg.DataSource == nil {
		return nil, fmt.Errorf("terraform_datasource step requires a `datasource \"<kind>\" { ... }` block")
	}
	args, err := engine.EvalBodyToMap(cfg.DataSource.Body, ctx, "terraform_datasource args")
	if err != nil {
		return nil, err
	}
	return NewDataSourceStep(collector, cfg.DataSource.Kind, args), nil
}
