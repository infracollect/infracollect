package runner

import (
	tfclient "github.com/adrien-f/tf-data-client"
	"github.com/go-logr/zapr"
	httpCollector "github.com/infracollect/infracollect/internal/collectors/http"
	"github.com/infracollect/infracollect/internal/collectors/terraform"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/infracollect/infracollect/internal/engine/steps"
	"github.com/samber/do/v2"
	"go.uber.org/zap"
)

// BuildContainer creates a new DI container with all dependencies registered.
// Dependencies are lazily initialized when first requested.
func BuildContainer(logger *zap.Logger) *do.RootScope {
	injector := do.New()

	// Register logger (eager - already created)
	do.ProvideValue(injector, logger)

	// Register terraform client (lazy - only created when first terraform collector is used)
	do.Provide(injector, func(i do.Injector) (terraform.Client, error) {
		log := do.MustInvoke[*zap.Logger](i)
		return tfclient.New(tfclient.WithLogger(zapr.NewLogger(log.Named("tfclient"))))
	})

	return injector
}

// BuildRegistry creates a new registry with all collectors and steps registered.
func BuildRegistry() *engine.Registry {
	registry := engine.NewRegistry()

	// Register collectors
	terraform.Register(registry)
	httpCollector.Register(registry)

	// Register steps
	steps.Register(registry)

	return registry
}
