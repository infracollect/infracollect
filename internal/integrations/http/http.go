package http

import (
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/infracollect/infracollect/internal/engine"
)

// CollectorConfig is the HCL-level shape of a `collector "http" "<id>" { ... }` block.
//
//	collector "http" "github" {
//	  base_url = "https://api.github.com"
//	  timeout  = 30
//	  headers = {
//	    X-GitHub-Api-Version = "2022-11-28"
//	  }
//	  auth "basic" {
//	    username = env.GITHUB_USER
//	    password = env.GITHUB_TOKEN
//	  }
//	}
type CollectorConfig struct {
	BaseURL  string            `hcl:"base_url"`
	Headers  map[string]string `hcl:"headers,optional"`
	Timeout  *int              `hcl:"timeout,optional"`
	Insecure bool              `hcl:"insecure,optional"`
	Auth     *AuthBlock        `hcl:"auth,block"`
}

// AuthBlock is a labeled block whose label selects the auth scheme. Today
// only `basic` is supported; future schemes (bearer, oauth) add new label
// cases without breaking existing configs.
type AuthBlock struct {
	Kind     string `hcl:"kind,label"`
	Username string `hcl:"username,optional"`
	Password string `hcl:"password,optional"`
	Encoded  string `hcl:"encoded,optional"`
}

// GetStepConfig is the HCL-level shape of a `step "http_get" "<id>" { ... }` block.
type GetStepConfig struct {
	Path         string            `hcl:"path"`
	Headers      map[string]string `hcl:"headers,optional"`
	Params       map[string]string `hcl:"params,optional"`
	ResponseType string            `hcl:"response_type,optional"`
}

func Register(registry *engine.Registry) error {
	if err := registry.RegisterCollector(
		CollectorKind,
		engine.NewCollectorFactory(CollectorKind, newCollector),
	); err != nil {
		return err
	}

	return registry.RegisterSteps(
		engine.NewTypedStepDescriptor(GetStepKind, CollectorKind, newGetStep),
	)
}

func newCollector(
	_ *engine.RegistryHelper,
	_ *hcl.EvalContext,
	cfg CollectorConfig,
) (engine.Collector, error) {
	c := Config{
		BaseURL:  cfg.BaseURL,
		Headers:  cfg.Headers,
		Insecure: cfg.Insecure,
	}

	if cfg.Auth != nil {
		switch cfg.Auth.Kind {
		case "basic":
			c.Auth = &AuthConfig{
				Basic: &BasicAuthConfig{
					Username: cfg.Auth.Username,
					Password: cfg.Auth.Password,
					Encoded:  cfg.Auth.Encoded,
				},
			}
		}
	}

	if cfg.Timeout != nil {
		c.Timeout = time.Duration(*cfg.Timeout) * time.Second
	}

	return NewCollector(c)
}

func newGetStep(
	_ *engine.RegistryHelper,
	_ string,
	collector *Collector,
	_ *hcl.EvalContext,
	cfg GetStepConfig,
) (engine.Step, error) {
	return NewGetStep(collector, GetConfig(cfg))
}
