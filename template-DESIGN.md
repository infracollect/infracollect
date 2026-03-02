# Template System Redesign: DAG-Based Lazy Evaluation

## Problem Statement

The current templating system expands all templates **before** pipeline creation, which prevents:

1. **Step output references** - Cannot use `${steps.fetch_config.data.endpoint}` in later steps
2. **Dynamic for_each** - Cannot iterate over data fetched by a previous step
3. **Conditional execution** - Cannot skip steps based on previous results
4. **Late-bound configuration** - All values must be known at job parse time

### Current Flow

```
Parse YAML → Expand ALL templates → Create Pipeline → Execute Steps
                ↑
        Too early! Step results
        don't exist yet.
```

### Desired Flow

```
Parse YAML → Build DAG → Execute in order → Expand templates just before each node
                              ↓
                    Previous results available
                    for template expansion
```

## Design Goals

1. **Consistent syntax** - All references use scoped namespaces (`env.`, `job.`, `steps.`, `each.`)
2. **Explicit dependencies** - Clear error messages for missing/circular deps
3. **Fail fast** - Detect cycles and missing refs at parse time, not runtime
4. **Testable** - Each component independently testable

## Template Syntax

**Breaking Change**: All references are now scoped with a namespace prefix for consistency and clarity.

### Reference Namespaces

| Namespace | Description           | Example                        |
| --------- | --------------------- | ------------------------------ |
| `env.`    | Environment variables | `${env.API_KEY}`               |
| `job.`    | Job metadata          | `${job.name}`, `${job.date}`   |
| `steps.`  | Step outputs          | `${steps.fetch.data.url}`      |
| `each.`   | For_each iteration    | `${each.key}`, `${each.value}` |

### Example Job

```yaml
apiVersion: v1
kind: CollectJob
metadata:
  name: my-job
spec:
  collectors:
    - id: api
      http:
        base_url: https://${env.API_HOST}/v1
        headers:
          Authorization: Bearer ${env.API_TOKEN}

  steps:
    - id: get_config
      collector: api
      http_get:
        path: /config

    - id: fetch_data
      collector: api
      http_get:
        # Reference previous step's output
        path: /data/${steps.get_config.data.endpoint}
        headers:
          X-Config-Version: ${steps.get_config.data.version}

  output:
    sink:
      filesystem:
        prefix: ${job.name}-${job.date}-
```

We'll use go-expr for template evaluation.

### Reference Examples

| Reference                                 | Description                            |
| ----------------------------------------- | -------------------------------------- |
| `${env.API_KEY}`                          | Environment variable `API_KEY`         |
| `${env.HOME}`                             | Environment variable `HOME`            |
| `${job.name}`                             | Job metadata name from `metadata.name` |
| `${job.date}`                             | Job execution timestamp (RFC3339)      |
| `${steps.fetch.data}`                     | Entire data payload from step "fetch"  |
| `${steps.fetch.data.items}`               | Field "items" from step data           |
| `${steps.fetch.data.items[0]}`            | First element of array                 |
| `${steps.fetch.data.config.nested.value}` | Nested field access                    |
| `${steps.fetch.meta.status_code}`         | Step metadata field                    |
| `${steps.fetch.id}`                       | Step ID (always equals "fetch")        |
| `${each.key}`                             | Current for_each iteration key         |
| `${each.value}`                           | Current for_each iteration value       |
| `${each.value.region}`                    | Field from for_each value object       |
