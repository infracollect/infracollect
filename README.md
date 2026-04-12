<p align="center"><img src="./website/public/full-logo.png" alt="infracollect" /></p>

**Collect infrastructure data from anywhere** — cloud providers, Kubernetes clusters, REST APIs — all with simple HCL
configuration.

infracollect lets you query and export data from your infrastructure using declarative job definitions.
It leverages the Terraform provider ecosystem to provide a near-infinite number of data sources on day one.
Specific collectors will be added over time to support more data sources natively.

## Why infracollect?

- **Infinite data sources** — Use Terraform providers, HTTP APIs, local files and more to collect data
- **Declarative HCL** — Define what to collect, not how to collect it
- **Flexible output** — Write to stdout, local files, or S3-compatible storage; optionally bundle results into a single
  `.tar.gz` or `.tar.zst` archive

## Quick Start

### Installation

```bash
go install github.com/infracollect/infracollect/cmd/infracollect@latest
```

### Your First Collection Job

Create a file called `job.hcl`:

```hcl
collector "terraform" "k8s" {
  provider    = "hashicorp/kubernetes"
  config_path = "~/.kube/config"
}

step "terraform_datasource" "deployments" {
  collector = collector.terraform.k8s
  datasource "kubernetes_resources" {
    api_version = "apps/v1"
    kind        = "Deployment"
    namespace   = "default"
  }
}
```

Run it:

```bash
infracollect collect job.hcl
```

The collected data is printed to stdout as JSON.

## Project Status

infracollect is in **early development**. The core functionality works, but APIs may change. Feedback and contributions
are welcome although we'd appreciate if you could discuss changes in the
[Discussions](https://github.com/infracollect/infracollect/discussions) first.

## Contributing

See [AGENTS.md](AGENTS.md) for code style and implementation guidelines.

## License

[License information to be added]
