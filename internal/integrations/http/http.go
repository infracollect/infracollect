package http

import (
	"context"
	"time"

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
		GetStepKind,
		engine.NewStepFactory(GetStepKind, newGetStep),
	)
}

func newCollector(_ context.Context, _ *zap.Logger, spec v1.HTTPCollector) (engine.Collector, error) {
	cfg := Config{
		BaseURL:  spec.BaseURL,
		Headers:  spec.Headers,
		Insecure: spec.Insecure,
	}

	if spec.Auth != nil && spec.Auth.Basic != nil {
		cfg.Auth = &AuthConfig{
			Basic: &BasicAuthConfig{
				Username: spec.Auth.Basic.Username,
				Password: spec.Auth.Basic.Password,
				Encoded:  spec.Auth.Basic.Encoded,
			},
		}
	}

	if spec.Timeout != nil {
		cfg.Timeout = time.Duration(*spec.Timeout) * time.Second
	}

	return NewCollector(cfg)
}

func newGetStep(_ context.Context, _ *zap.Logger, _ string, collector *Collector, spec v1.HTTPGetStep) (engine.Step, error) {
	return NewGetStep(collector, GetConfig{
		Path:         spec.Path,
		Headers:      spec.Headers,
		Params:       spec.Params,
		ResponseType: spec.ResponseType,
	})
}
