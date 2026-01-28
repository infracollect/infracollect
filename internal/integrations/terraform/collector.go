package terraform

import (
	"context"
	"fmt"
	"strings"

	tfaddr "github.com/hashicorp/terraform-registry-address"
	"github.com/infracollect/infracollect/internal/engine"
	tfclient "github.com/infracollect/tf-data-client"
)

const (
	CollectorKind = "terraform"
)

// Client is an interface for creating and managing Terraform providers.
type Client interface {
	CreateProvider(ctx context.Context, config tfclient.ProviderConfig) (tfclient.Provider, error)
	StopProvider(ctx context.Context, config tfclient.ProviderConfig) error
}

type Config struct {
	Provider string
	Version  string
	Args     map[string]any
}

type Collector struct {
	providerConfig tfclient.ProviderConfig
	provider       tfclient.Provider
	args           map[string]any
	client         Client
}

func NewCollector(client Client, cfg Config) (engine.Collector, error) {
	provider, err := tfaddr.ParseProviderSource(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to parse provider source '%s': %w", cfg.Provider, err)
	}

	version := strings.TrimPrefix(cfg.Version, "v")

	return &Collector{
		providerConfig: tfclient.ProviderConfig{
			Namespace: provider.Namespace,
			Name:      provider.Type,
			Version:   version,
		},
		args:   cfg.Args,
		client: client,
	}, nil
}

func (c *Collector) Name() string {
	return fmt.Sprintf("%s(%s)", CollectorKind, c.providerConfig.String())
}

func (c *Collector) Kind() string {
	return CollectorKind
}

func (c *Collector) Start(ctx context.Context) error {
	if c.provider != nil {
		return nil
	}

	provider, err := c.client.CreateProvider(ctx, c.providerConfig)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	if err := provider.Configure(ctx, c.args); err != nil {
		return fmt.Errorf("failed to configure provider: %w", err)
	}

	c.provider = provider
	return nil
}

func (c *Collector) ReadDataSource(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("provider not started")
	}

	if !c.provider.IsConfigured() {
		return nil, fmt.Errorf("provider not configured")
	}

	result, err := c.provider.ReadDataSource(ctx, name, args)
	if err != nil {
		return nil, fmt.Errorf("failed to read data source: %w", err)
	}

	return result.State, nil
}

func (c *Collector) Close(ctx context.Context) error {
	return c.client.StopProvider(ctx, c.providerConfig)
}

func (c *Collector) ProviderSource() string {
	var (
		namespace string
		name      string
	)
	if c.provider != nil {
		namespace = c.provider.Config().Namespace
		name = c.provider.Config().Name
	} else {
		namespace = c.providerConfig.Namespace
		name = c.providerConfig.Name
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (c *Collector) ProviderVersion() string {
	var version string
	if c.provider != nil {
		version = c.provider.Config().Version
	} else {
		version = c.providerConfig.Version
	}
	return version
}
