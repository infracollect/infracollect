---
title: Your first job
description: Create your first job file
---

Create a `job.yaml` file:

```yaml
kind: CollectJob
metadata:
  name: test
spec:
  collectors:
    - id: kind
      terraform:
        provider: hashicorp/kubernetes
        args:
          config_path: ~/.kube/config
          config_context: kind-kind
  steps:
    - id: deployments
      collector: kind
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: apps/v1
          kind: Deployment
          namespace: kube-system
```

And run the job:

```bash
infracollect collect job.yaml
```
