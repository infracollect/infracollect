---
title: Your first job
description: Create your first job file
---

Create a `job.hcl` file:

```hcl
collector "terraform" "kind" {
  provider       = "hashicorp/kubernetes"
  config_path    = "~/.kube/config"
  config_context = "kind-kind"
}

step "terraform_datasource" "deployments" {
  collector = collector.terraform.kind
  datasource "kubernetes_resources" {
    api_version = "apps/v1"
    kind        = "Deployment"
    namespace   = "kube-system"
  }
}
```

And run the job:

```bash
infracollect collect job.hcl
```
