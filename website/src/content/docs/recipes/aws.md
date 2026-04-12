---
title: AWS
description: Collect data from AWS
---

Collect data from AWS:

```hcl
collector "terraform" "aws" {
  provider = "hashicorp/aws"
  region   = "us-east-1"
}

step "terraform_datasource" "ec2-instances" {
  collector = collector.terraform.aws
  datasource "aws_instances" {}
}
```

And run the job:

```bash
infracollect collect job.hcl
```
