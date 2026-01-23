package terraform

import (
	"context"
	"errors"
	"testing"

	tfclient "github.com/adrien-f/tf-data-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	configureFunc      func(ctx context.Context, args map[string]any) error
	readDataSourceFunc func(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error)
	isConfigured       bool
}

func (m *mockProvider) Configure(ctx context.Context, args map[string]any) error {
	if m.configureFunc != nil {
		return m.configureFunc(ctx, args)
	}
	m.isConfigured = true
	return nil
}

func (m *mockProvider) ReadDataSource(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error) {
	if m.readDataSourceFunc != nil {
		return m.readDataSourceFunc(ctx, name, args)
	}
	return &tfclient.DataSourceResult{State: map[string]any{}}, nil
}

func (m *mockProvider) IsConfigured() bool {
	return m.isConfigured
}

func (m *mockProvider) ListDataSources() []string {
	return nil
}

func (m *mockProvider) Close() error {
	return nil
}

type mockClient struct {
	createProviderFunc func(ctx context.Context, config tfclient.ProviderConfig) (tfclient.Provider, error)
	stopProviderFunc   func(ctx context.Context, config tfclient.ProviderConfig) error
	provider           *mockProvider
}

func (m *mockClient) CreateProvider(ctx context.Context, config tfclient.ProviderConfig) (tfclient.Provider, error) {
	if m.createProviderFunc != nil {
		return m.createProviderFunc(ctx, config)
	}
	if m.provider == nil {
		m.provider = &mockProvider{}
	}
	return m.provider, nil
}

func (m *mockClient) StopProvider(ctx context.Context, config tfclient.ProviderConfig) error {
	if m.stopProviderFunc != nil {
		return m.stopProviderFunc(ctx, config)
	}
	return nil
}

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid aws provider",
			cfg: Config{
				Provider: "hashicorp/aws",
				Version:  "5.0.0",
				Args:     map[string]any{"region": "us-east-1"},
			},
			wantErr: false,
		},
		{
			name: "valid provider with v prefix",
			cfg: Config{
				Provider: "hashicorp/kubernetes",
				Version:  "v2.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid provider without version",
			cfg: Config{
				Provider: "hashicorp/google",
			},
			wantErr: false,
		},
		{
			name: "invalid provider source",
			cfg: Config{
				Provider: "invalid provider",
			},
			wantErr:     true,
			errContains: "failed to parse provider source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{}
			collector, err := NewCollector(client, tt.cfg)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, collector)
		})
	}
}

func TestCollector_NameAndKind(t *testing.T) {
	client := &mockClient{}
	collector, err := NewCollector(client, Config{
		Provider: "hashicorp/aws",
		Version:  "5.0.0",
	})
	require.NoError(t, err)

	assert.Equal(t, "terraform(hashicorp/aws@5.0.0)", collector.Name())
	assert.Equal(t, "terraform", collector.Kind())
}

func TestCollector_Start(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *mockClient
		wantErr     bool
		errContains string
	}{
		{
			name: "successful start",
			setupClient: func() *mockClient {
				return &mockClient{
					provider: &mockProvider{},
				}
			},
			wantErr: false,
		},
		{
			name: "create provider fails",
			setupClient: func() *mockClient {
				return &mockClient{
					createProviderFunc: func(ctx context.Context, config tfclient.ProviderConfig) (tfclient.Provider, error) {
						return nil, errors.New("provider not found")
					},
				}
			},
			wantErr:     true,
			errContains: "failed to create provider",
		},
		{
			name: "configure provider fails",
			setupClient: func() *mockClient {
				return &mockClient{
					provider: &mockProvider{
						configureFunc: func(ctx context.Context, args map[string]any) error {
							return errors.New("invalid credentials")
						},
					},
				}
			},
			wantErr:     true,
			errContains: "failed to configure provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			collector, err := NewCollector(client, Config{
				Provider: "hashicorp/aws",
				Args:     map[string]any{"region": "us-east-1"},
			})
			require.NoError(t, err)

			err = collector.Start(t.Context())

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestCollector_Start_Idempotent(t *testing.T) {
	callCount := 0
	client := &mockClient{
		createProviderFunc: func(ctx context.Context, config tfclient.ProviderConfig) (tfclient.Provider, error) {
			callCount++
			return &mockProvider{}, nil
		},
	}

	collector, err := NewCollector(client, Config{Provider: "hashicorp/aws"})
	require.NoError(t, err)

	// First start
	err = collector.Start(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second start should be a no-op
	err = collector.Start(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestCollector_ReadDataSource(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *mockClient
		started     bool
		wantErr     bool
		errContains string
		wantData    map[string]any
	}{
		{
			name: "successful read",
			setupClient: func() *mockClient {
				return &mockClient{
					provider: &mockProvider{
						isConfigured: true,
						readDataSourceFunc: func(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error) {
							return &tfclient.DataSourceResult{
								State: map[string]any{
									"id":   "i-12345",
									"name": "test-instance",
								},
							}, nil
						},
					},
				}
			},
			started: true,
			wantErr: false,
			wantData: map[string]any{
				"id":   "i-12345",
				"name": "test-instance",
			},
		},
		{
			name: "provider not started",
			setupClient: func() *mockClient {
				return &mockClient{}
			},
			started:     false,
			wantErr:     true,
			errContains: "provider not started",
		},
		{
			name: "provider not configured",
			setupClient: func() *mockClient {
				return &mockClient{
					provider: &mockProvider{
						isConfigured: false,
						configureFunc: func(ctx context.Context, args map[string]any) error {
							// Don't set isConfigured to true
							return nil
						},
					},
				}
			},
			started:     true,
			wantErr:     true,
			errContains: "provider not configured",
		},
		{
			name: "read data source fails",
			setupClient: func() *mockClient {
				return &mockClient{
					provider: &mockProvider{
						isConfigured: true,
						readDataSourceFunc: func(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error) {
							return nil, errors.New("data source not found")
						},
					},
				}
			},
			started:     true,
			wantErr:     true,
			errContains: "failed to read data source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			collector, err := NewCollector(client, Config{Provider: "hashicorp/aws"})
			require.NoError(t, err)

			if tt.started {
				err = collector.Start(t.Context())
				require.NoError(t, err)
			}

			c := collector.(*Collector)
			data, err := c.ReadDataSource(t.Context(), "aws_instance", map[string]any{"id": "i-12345"})

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, data)
		})
	}
}

func TestCollector_Close(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *mockClient
		wantErr     bool
	}{
		{
			name: "successful close",
			setupClient: func() *mockClient {
				return &mockClient{}
			},
			wantErr: false,
		},
		{
			name: "stop provider fails",
			setupClient: func() *mockClient {
				return &mockClient{
					stopProviderFunc: func(ctx context.Context, config tfclient.ProviderConfig) error {
						return errors.New("failed to stop")
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			collector, err := NewCollector(client, Config{Provider: "hashicorp/aws"})
			require.NoError(t, err)

			err = collector.Close(t.Context())

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
