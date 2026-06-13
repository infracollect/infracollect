---
title: Preview a job
description: Inspect a job's execution plan before running collectors or steps
---

Before running a job — especially against live infrastructure — you can preview
exactly what infracollect will do. The `plan` command parses the job, builds the
execution DAG, and prints the resolved plan **without** starting any collector or
running any step.

```bash
infracollect plan job.hcl
```

## Reading the plan

Given this job:

```hcl
job {
  name = "infra-snapshot"
}

step "static" "regions" {
  value = "[\"eu-west-1\",\"us-east-1\"]"
}

step "static" "per_region" {
  for_each = jsondecode(step.static.regions.data)
  value    = each.value
}

output {
  encoding "json" {}
  sink "filesystem" {
    path = "./out"
  }
  steps = [step.static.per_region]
}
```

`infracollect plan job.hcl` prints:

```text
Job:  infra-snapshot

  1.  [step]        static/regions
  2.  [collection]  static/per_region  (for_each: computed at runtime)
                    depends on:        static/regions

Output:
  encoding:  json
  sink:      filesystem
  steps:     static/per_region
```

Nodes are listed in the order they will execute (a topological sort of the DAG):

- The tag in brackets is the node kind — `collector`, `step`, or `collection`
  (a `step` with `for_each`).
- `depends on` lists the upstream nodes whose results a node references.
- `collector` (when shown) is the collector a step is bound to.
- The `Output` block echoes the resolved encoding, sink, optional archive, and
  the `steps` filter when one is set.

## `for_each` resolution

When a `for_each` expression depends only on literals, environment variables, or
job variables, infracollect evaluates it at plan time and shows the iteration
keys:

```text
  1.  [collection]  static/report  (for_each: 2 → eu-west-1, us-east-1)
```

When the expression depends on upstream collector or step data — like the
`jsondecode(step.static.regions.data)` above — the iteration set is only known
once those nodes run, so the plan reports `computed at runtime`.
