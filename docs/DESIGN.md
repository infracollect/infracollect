# infracollect Design

## Project Goals

infracollect is a tool designed to collect infrastructure and cloud resources using the vast ecosystem of Terraform providers. The project leverages the `tf-data-client` library to directly run Terraform providers as Go libraries, without needing the Terraform or OpenTofu CLI.

## Core Objectives

1. **Provider-Agnostic Collection**: Collect resources from any infrastructure provider that has a Terraform provider
2. **Declarative Configuration**: Define collection jobs using simple YAML configuration
3. **Multi-Collector Support**: Support multiple collectors in a single job, each with different providers/configurations
4. **Pluggable Architecture**: Design for extensibility with interfaces that allow swapping implementations
5. **Direct Provider Execution**: Use `tf-data-client` library to run Terraform providers directly without CLI overhead

## Why tf-data-client?

The `tf-data-client` library provides a direct way to run Terraform providers as Go libraries:

- **No CLI Required**: Direct library integration eliminates the need for Terraform/OpenTofu CLI
- **Efficient**: No subprocess overhead or HCL generation needed
- **Native Go**: Full Go integration with type safety and error handling
- **Provider Compatibility**: Works with any Terraform provider
- **Simplified Architecture**: Fewer moving parts compared to CLI-based approaches

## Design Principles

### 1. Pluggability

All major components are defined as interfaces in `pkg/engine/`, allowing for:
- Different implementations of collectors (currently terraform, extensible to others)
- Custom step types
- Pluggable data collection strategies

### 2. Extensibility

The system is designed to be extended without modification:
- New providers can be added by simply referencing them in YAML (they work automatically via `tf-data-client`)
- Custom collectors can be implemented by following the `Collector` interface
- Custom step types can be added by implementing the `Step` interface

### 3. Provider-Agnostic

The system doesn't need to know about specific providers. It:
- Uses `tf-data-client` to handle provider initialization and configuration
- Executes data sources through the provider library
- Returns collected data in a standardized format (map[string]any)

### 4. Isolation

Each collector operates with its own provider instance:
- Independent provider configuration and state
- No cross-contamination between collectors
- Each collector manages its own provider lifecycle

## Core Concepts

### CollectJob

A `CollectJob` is a YAML definition that describes:
- **Collectors**: Provider instances with their configuration
- **Steps**: Data collection operations that reference collectors and data sources

Example:
```yaml
kind: CollectJob
metadata:
  name: k8s-deployments
spec:
  collectors:
    - id: kind
      terraform:
        provider: hashicorp/kubernetes
        version: v2.32.0
        args:
          config_path: ./kubeconfig
          config_context: kind-kind
  steps:
    - id: deployments
      terraform_datasource:
        name: kubernetes_resources
        collector: kind
        args:
          api_version: apps/v1
          kind: Deployment
          namespace: kube-system
```

### Collectors

A **Collector** represents an instance of a Terraform provider with specific configuration:
- Each collector has a unique ID
- Contains provider name, version, and arguments
- Manages its own provider instance via `tf-data-client`
- Can be referenced by multiple steps

### Steps

A **Step** represents a data collection operation:
- References a collector by ID
- Specifies a Terraform data source to query
- Provides arguments for the data source
- Produces collected resource data

### DataSources

A **DataSource** is a Terraform data source that queries infrastructure:
- Examples: `kubernetes_resources`, `aws_instances`, `azurerm_resources`
- Each data source has specific arguments
- Returns resource data in JSON format

## Multi-Collector Support

The system supports multiple collectors in a single job:

- **Different Providers**: Each collector can use a different provider
- **Different Configurations**: Same provider, different configs (e.g., different AWS regions)
- **Shared Usage**: Multiple steps can use the same collector
- **Isolated State**: Each collector maintains its own provider instance and state

## Use Cases

1. **Multi-Cloud Inventory**: Collect resources from AWS, Azure, and GCP in a single pipeline
2. **Multi-Region Collection**: Collect resources from multiple AWS regions
3. **Kubernetes Multi-Cluster**: Collect resources from multiple Kubernetes clusters
4. **Hybrid Infrastructure**: Combine cloud and on-premises resources
5. **Resource Discovery**: Discover and catalog all infrastructure resources

## Future Considerations

- **Output Formats**: Add structured output handlers (JSON, YAML, CSV, etc.)
- **Plugin System**: Allow custom collectors and step types via plugins
- **Scheduling**: Support scheduled collection jobs
- **Caching**: Cache collected data to reduce provider API calls
- **Filtering**: Add filtering capabilities for collected resources
- **Parallel Step Execution**: Execute independent steps concurrently
