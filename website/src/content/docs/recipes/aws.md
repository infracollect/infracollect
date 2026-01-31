---
title: AWS
description: Collect data from AWS
---

Collect data from AWS:

```yaml
apiVersion: v1
kind: CollectJob
metadata:
  name: aws
spec:
  collectors:
    - id: aws
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1
  steps:
    - id: ec2-instances
      collector: aws
      terraform_datasource:
        name: aws_instances
        args: {}
```

And run the job:

```bash
infracollect collect job.yaml
```
