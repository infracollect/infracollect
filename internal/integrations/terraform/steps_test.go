package terraform

import (
	"context"
	"errors"
	"testing"

	tfclient "github.com/infracollect/tf-data-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataSourceStep_Resolve(t *testing.T) {
	tests := []struct {
		name        string
		dsName      string
		args        map[string]any
		setupMock   func() *mockProvider
		wantErr     bool
		errContains string
		wantData    map[string]any
		wantMeta    map[string]string
	}{
		{
			name:   "success",
			dsName: "aws_instance",
			args:   map[string]any{"id": "i-12345"},
			setupMock: func() *mockProvider {
				return &mockProvider{
					isConfigured: true,
					providerConfig: tfclient.ProviderConfig{
						Namespace: "hashicorp",
						Name:      "aws",
						Version:   "5.0.0",
					},
					readDataSourceFunc: func(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error) {
						return &tfclient.DataSourceResult{
							State: map[string]any{
								"id":            "i-12345",
								"instance_type": "t3.micro",
							},
						}, nil
					},
				}
			},
			wantData: map[string]any{
				"id":            "i-12345",
				"instance_type": "t3.micro",
			},
			wantMeta: map[string]string{
				"terraform_provider":         "hashicorp/aws",
				"terraform_provider_version": "5.0.0",
				"terraform_datasource":       "aws_instance",
			},
		},
		{
			name:   "error from collector",
			dsName: "aws_instance",
			args:   map[string]any{"id": "i-99999"},
			setupMock: func() *mockProvider {
				return &mockProvider{
					isConfigured: true,
					readDataSourceFunc: func(ctx context.Context, name string, args map[string]any) (*tfclient.DataSourceResult, error) {
						return nil, errors.New("instance not found")
					},
				}
			},
			wantErr:     true,
			errContains: "instance not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProv := tt.setupMock()
			client := &mockClient{provider: mockProv}

			collector, err := NewCollector(client, Config{Provider: "hashicorp/aws"})
			require.NoError(t, err)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			step := NewDataSourceStep(collector.(*Collector), tt.dsName, tt.args)

			result, err := step.Resolve(t.Context())

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantData, result.Data)
			assert.Equal(t, tt.wantMeta, result.Meta)
		})
	}
}

func TestDataSourceStep_NameAndKind(t *testing.T) {
	client := &mockClient{provider: &mockProvider{}}
	collector, err := NewCollector(client, Config{Provider: "hashicorp/aws"})
	require.NoError(t, err)
	step := NewDataSourceStep(collector.(*Collector), "aws_instance", nil)

	assert.Equal(t, "terraform_datasource(aws_instance)", step.Name())
	assert.Equal(t, "terraform_datasource", step.Kind())
}
