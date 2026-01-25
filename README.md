<center><img src="./website/public/full-logo.png" alt="infracollect" /></center>

**Collect infrastructure data from anywhere** — cloud providers, Kubernetes clusters, REST APIs — all with simple YAML configuration.

infracollect lets you query and export data from your infrastructure using declarative job definitions. It leverages the entire Terraform provider ecosystem without requiring the Terraform CLI, and supports HTTP APIs for maximum flexibility.

## Why infracollect?

- **No Terraform CLI required** — Runs providers directly as Go libraries
- **Multi-source collection** — Combine data from AWS, Azure, GCP, Kubernetes, and REST APIs in a single job
- **Declarative YAML** — Define what to collect, not how to collect it
- **Flexible output** — Write to stdout, local files, or S3-compatible storage; optionally bundle results into a single `.tar.gz` or `.tar.zst` archive

## Quick Start

### Installation

```bash
go install github.com/adrien-f/infracollect/cmd/infracollect@latest
```

### Your First Collection Job

Create a file called `job.yaml`:

```yaml
kind: CollectJob
metadata:
  name: my-first-job
spec:
  collectors:
    - id: k8s
      terraform:
        provider: hashicorp/kubernetes
        args:
          config_path: ~/.kube/config

  steps:
    - id: deployments
      collector: k8s
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: apps/v1
          kind: Deployment
          namespace: default
```

Run it:

```bash
infracollect collect job.yaml
```

The collected data is printed to stdout as JSON.

## Examples

### Collect from a REST API

```yaml
kind: CollectJob
metadata:
  name: api-data
spec:
  collectors:
    - id: api
      http:
        base_url: https://api.example.com
        auth:
          basic:
            username: ${API_USER}
            password: ${API_PASSWORD}

  steps:
    - id: users
      collector: api
      http_get:
        path: /users

    - id: orders
      collector: api
      http_get:
        path: /orders
        params:
          status: active
```

### Multi-Cloud Inventory

Collect resources from multiple cloud providers in a single job:

```yaml
kind: CollectJob
metadata:
  name: multi-cloud
spec:
  collectors:
    - id: aws
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1

    - id: azure
      terraform:
        provider: hashicorp/azurerm
        args:
          subscription_id: ${AZURE_SUBSCRIPTION_ID}
          tenant_id: ${AZURE_TENANT_ID}

  steps:
    - id: ec2-instances
      collector: aws
      terraform_datasource:
        name: aws_instances
        args: {}

    - id: azure-vms
      collector: azure
      terraform_datasource:
        name: azurerm_virtual_machines
        args:
          resource_group_name: production
```

### Save Output to Files

Write results to local files, organized by job name and timestamp:

```yaml
spec:
  # ... collectors and steps ...

  output:
    encoding:
      json:
        indent: "  " # Pretty-print the JSON
    sink:
      filesystem:
        path: ./output
        prefix: $JOB_NAME/$JOB_DATE_ISO8601
```

This creates files like `./output/my-job/20260123T120000Z/deployments.json`.

### Bundle Output into an Archive

Bundle all step outputs into a single `.tar.gz` or `.tar.zst` file (requires a filesystem or S3 sink):

```yaml
spec:
  # ... collectors and steps ...

  output:
    encoding:
      json:
        indent: "  "
    archive:
      format: tar
      compression: gzip # or zstd, none
      name: $JOB_NAME-$JOB_DATE_ISO8601
    sink:
      filesystem:
        path: ./output
        prefix: $JOB_NAME/$JOB_DATE_RFC3339
```

This produces one file like `./output/my-job/2026-01-24T12:00:00Z/my-job-20260124T120000Z.tar.gz` containing `users.json`, `posts.json`, etc.

### Export to S3

Write results directly to S3, R2, or MinIO:

```yaml
spec:
  # ... collectors and steps ...

  output:
    sink:
      s3:
        bucket: my-exports-bucket
        region: us-west-2
        prefix: infracollect/$JOB_NAME/$JOB_DATE_ISO8601
```

## Key Concepts

| Concept        | Description                                                                                       |
| -------------- | ------------------------------------------------------------------------------------------------- |
| **CollectJob** | A YAML file defining what data to collect and where to send it                                    |
| **Collector**  | A data source configuration (Terraform provider or HTTP client)                                   |
| **Step**       | A single data collection operation using a collector                                              |
| **Output**     | Where and how to write collected data (stdout, filesystem, S3); optional archive (tar.gz/tar.zst) |

## Supported Collectors

### Terraform Providers

Use any Terraform provider to collect infrastructure data:

- **AWS** (`hashicorp/aws`) — EC2, S3, RDS, Lambda, etc.
- **Azure** (`hashicorp/azurerm`) — VMs, Storage, AKS, etc.
- **GCP** (`hashicorp/google`) — Compute, GKE, Cloud Storage, etc.
- **Kubernetes** (`hashicorp/kubernetes`) — Pods, Deployments, Services, etc.
- **And many more** — Any Terraform provider with data sources works

### HTTP

Query any REST API with built-in support for:

- Basic authentication
- Custom headers
- Query parameters
- JSON or raw response parsing

## Documentation

| Document                                           | Description                       |
| -------------------------------------------------- | --------------------------------- |
| [Job Specification](docs/COLLECT_PIPELINE_SPEC.md) | Complete YAML schema reference    |
| [Design](docs/DESIGN.md)                           | Architecture and design decisions |
| [Architecture](docs/ARCHITECTURE.md)               | System components                 |

## Project Status

infracollect is in **early development**. The core functionality works, but APIs may change. Feedback and contributions are welcome.

## Contributing

See [AGENTS.md](AGENTS.md) for code style and implementation guidelines.

## License

[License information to be added]
