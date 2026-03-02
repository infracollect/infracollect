package main

import (
	"fmt"

	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/steps"
	"github.com/infracollect/infracollect/internal/integrations/http"
	"github.com/infracollect/infracollect/internal/integrations/terraform"
	"go.uber.org/zap"
)

// buildRegistry wires up the default set of collectors and steps. It is the
// single place the CLI constructs an engine.Registry — both `collect` and
// `validate` share it so their surface areas never drift.
func buildRegistry(logger *zap.Logger, allowedEnv []string) (*engine.Registry, error) {
	registry := engine.NewRegistry(logger)
	registry.RegisterDependency(engine.AllowedEnvVarsDepKey, allowedEnv)

	if err := terraform.Register(registry); err != nil {
		return nil, fmt.Errorf("register terraform integration: %w", err)
	}
	if err := http.Register(registry); err != nil {
		return nil, fmt.Errorf("register http integration: %w", err)
	}
	if err := steps.Register(registry); err != nil {
		return nil, fmt.Errorf("register builtin steps: %w", err)
	}

	return registry, nil
}
