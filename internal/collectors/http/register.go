package http

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/infracollect/infracollect/apis/v1"
	"github.com/infracollect/infracollect/internal/engine"
	"github.com/samber/do/v2"
)

// Register registers the http collector and step factories with the registry.
func Register(r *engine.Registry) {
	r.RegisterCollector(CollectorKind, collectorFactory)
	r.RegisterStep(GetStepKind, getStepFactory)
}

func collectorFactory(ctx context.Context, i do.Injector, spec any) (engine.Collector, error) {
	httpSpec, ok := spec.(*v1.HTTPCollector)
	if !ok {
		return nil, fmt.Errorf("expected *v1.HTTPCollector, got %T", spec)
	}

	cfg := Config{
		BaseURL:  httpSpec.BaseURL,
		Headers:  httpSpec.Headers,
		Insecure: httpSpec.Insecure,
	}

	if httpSpec.Auth != nil && httpSpec.Auth.Basic != nil {
		cfg.Auth = &AuthConfig{
			Basic: &BasicAuthConfig{
				Username: httpSpec.Auth.Basic.Username,
				Password: httpSpec.Auth.Basic.Password,
				Encoded:  httpSpec.Auth.Basic.Encoded,
			},
		}
	}

	if httpSpec.Timeout != nil {
		cfg.Timeout = time.Duration(*httpSpec.Timeout) * time.Second
	}

	return NewCollector(cfg)
}

func getStepFactory(ctx context.Context, i do.Injector, collector engine.Collector, spec any) (engine.Step, error) {
	getSpec, ok := spec.(*v1.HTTPGetStep)
	if !ok {
		return nil, fmt.Errorf("expected *v1.HTTPGetStep, got %T", spec)
	}

	httpCollector, ok := collector.(*Collector)
	if !ok {
		return nil, fmt.Errorf("http_get step requires http collector, got %s", collector.Kind())
	}

	return NewGetStep(httpCollector, GetConfig{
		Path:         getSpec.Path,
		Headers:      getSpec.Headers,
		Params:       getSpec.Params,
		ResponseType: getSpec.ResponseType,
	})
}
