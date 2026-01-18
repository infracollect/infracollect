# infracollect

infracollect is a tool for collecting infrastructure and cloud resources using the vast ecosystem of Terraform providers through OpenTofu.

## Overview

infracollect allows you to define collection jobs in YAML that specify which Terraform providers to use and what resources to collect. It leverages the `tf-data-client` library to directly run Terraform providers and execute data source queries.

## Features

- **Provider-Agnostic**: Works with any Terraform provider
- **Declarative Configuration**: Define collection jobs in simple YAML
- **Multi-Collector Support**: Use multiple collectors with different providers/configurations in a single job
- **Direct Provider Execution**: Uses `tf-data-client` library to run Terraform providers directly (no CLI needed)

## Quick Start

### Installation

```bash
go install github.com/adrien-f/infracollect/cmd/infracollect@latest
```

### Example Job

Create a file `job.yaml`:

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

### Run

```bash
infracollect collect job.yaml
```

## Documentation

- **[Design](docs/DESIGN.md)**: Project goals, design principles, and core concepts
- **[Architecture](docs/ARCHITECTURE.md)**: System architecture and component breakdown
- **[Job Specification](docs/COLLECT_PIPELINE_SPEC.md)**: Complete YAML schema reference
- **[Agent Guidelines](AGENTS.md)**: Code style and implementation guidelines

## How It Works

infracollect uses the `tf-data-client` library to directly run Terraform providers as Go libraries, without needing the Terraform or OpenTofu CLI. This provides:
- Direct integration with Terraform providers
- No need for HCL generation or CLI subprocess execution
- Efficient provider lifecycle management

## Project Status

This project is in early development. The core architecture and interfaces are being established.

## Contributing

See [AGENTS.md](AGENTS.md) for code style and implementation guidelines.

## License

[License information to be added]
