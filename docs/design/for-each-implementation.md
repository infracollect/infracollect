# Implementation Plan: for_each for Collectors and Steps

This document outlines the implementation plan for adding `for_each` support to collectors and steps in CollectJob specifications.

## Design Decisions

| Decision | Choice |
|----------|--------|
| Expansion model | Static expansion at parse time |
| Syntax | Map-of-maps with `each.key` / `each.value` |
| Result keys | Separate keys for expanded steps |
| Cross-references | Explicit matching (no wildcards) |
| Templating | Expr library with `${}` syntax |
| Variable namespaces | Explicit: `env.*`, `var.*`, `each.*` |

## Target Syntax

### Spec-Level Variables

```yaml
kind: CollectJob
metadata:
  name: multi-env
spec:
  variables:
    environments:
      dev:
        context: dev-cluster
        region: us-east-1
      live:
        context: live-cluster
        region: eu-west-1
    team: platform
```

### Collector for_each

```yaml
collectors:
  - for_each:
      dev:
        context: dev-cluster
      live:
        context: live-cluster
    id: k8s_${each.key}
    terraform:
      provider: hashicorp/kubernetes
      args:
        context: ${each.value.context}
```

Expands to:

```yaml
collectors:
  - id: k8s_dev
    terraform:
      provider: hashicorp/kubernetes
      args:
        context: dev-cluster
  - id: k8s_live
    terraform:
      provider: hashicorp/kubernetes
      args:
        context: live-cluster
```

### Step for_each

```yaml
steps:
  - for_each:
      dev: {}
      live: {}
    id: namespaces_${each.key}
    collect:
      collector: k8s_${each.key}
      resource: kubernetes_namespace
```

Expands to separate steps with separate result keys:
- `namespaces_dev` → results from `k8s_dev`
- `namespaces_live` → results from `k8s_live`

### Referencing Spec Variables in for_each

```yaml
spec:
  variables:
    environments:
      dev: { context: dev-cluster }
      live: { context: live-cluster }

  collectors:
    - for_each: ${var.environments}
      id: k8s_${each.key}
      terraform:
        provider: hashicorp/kubernetes
        args:
          context: ${each.value.context}
```

## Variable Namespaces

All template expressions use explicit namespaces:

| Namespace | Source | Example |
|-----------|--------|---------|
| `env.*` | OS environment variables | `${env.HOME}` |
| `var.*` | `spec.variables` | `${var.team}` |
| `each.*` | Current for_each iteration | `${each.key}`, `${each.value.context}` |

The `each` namespace is only available within a `for_each` block.

## Implementation Steps

### Phase 1: Templating Engine with Expr

**File:** `internal/runner/template.go`

#### 1.1 Add expr dependency

```bash
go get github.com/expr-lang/expr
```

#### 1.2 Create interpolation context type

```go
type InterpolationContext struct {
    Env  map[string]string // OS environment (filtered)
    Var  map[string]any    // spec.variables
    Each *EachContext      // nil outside for_each
}

type EachContext struct {
    Key   string
    Value any
}
```

#### 1.3 Implement expr-based interpolation

```go
func Interpolate(template string, ctx InterpolationContext) (string, error)
```

- Parse `${...}` patterns from the string
- Extract expression inside `${}`
- Compile and evaluate with expr
- Replace with result
- Return error if expression is invalid or references undefined variable

#### 1.4 Handle expr compilation caching (optional optimization)

For repeated interpolation with same expressions but different `each` values, consider caching compiled programs.

### Phase 2: Spec Schema Updates

**File:** `api/v1/types.go` (or equivalent)

#### 2.1 Add Variables to Spec

```go
type CollectJobSpec struct {
    Variables  map[string]any `json:"variables,omitempty"`
    Collectors []Collector    `json:"collectors"`
    Steps      []Step         `json:"steps"`
}
```

#### 2.2 Add ForEach to Collector

```go
type Collector struct {
    ForEach   map[string]any `json:"for_each,omitempty"`
    ID        string         `json:"id" template:""`
    Terraform *TerraformConfig `json:"terraform,omitempty"`
    // ... other fields
}
```

#### 2.3 Add ForEach to Step

```go
type Step struct {
    ForEach map[string]any `json:"for_each,omitempty"`
    ID      string         `json:"id" template:""`
    Collect *CollectConfig `json:"collect,omitempty"`
    // ... other fields
}
```

### Phase 3: Expansion Logic

**File:** `internal/runner/expand.go` (new file)

#### 3.1 Implement collector expansion

```go
func ExpandCollectors(collectors []Collector, ctx InterpolationContext) ([]Collector, error)
```

For each collector:
1. If no `for_each`, interpolate templates and return as-is
2. If `for_each` is a string like `${var.environments}`, resolve it first
3. For each key/value in `for_each` map:
   - Clone the collector
   - Set `ctx.Each = &EachContext{Key: key, Value: value}`
   - Interpolate all template fields
   - Clear `ForEach` field on expanded collector
   - Append to result

#### 3.2 Implement step expansion

```go
func ExpandSteps(steps []Step, ctx InterpolationContext) ([]Step, error)
```

Same logic as collectors.

#### 3.3 Deep clone helper

```go
func deepClone[T any](v T) (T, error)
```

Use `encoding/json` marshal/unmarshal or a reflection-based approach.

### Phase 4: Integration into Job Parsing

**File:** `internal/runner/job.go` (or equivalent)

#### 4.1 Update parse pipeline

```
ParseYAML
    ↓
Build InterpolationContext (env + spec.variables)
    ↓
Resolve spec.variables templates (var can reference env)
    ↓
ExpandCollectors (handles for_each)
    ↓
ExpandSteps (handles for_each)
    ↓
Validate expanded spec
    ↓
Build Pipeline
```

#### 4.2 Environment variable filtering

Decide which env vars to expose:
- Option A: All env vars (simple but potentially leaky)
- Option B: Explicit allowlist in spec
- Option C: Prefix-based filtering (e.g., `INFRA_*`)

Recommend starting with Option A for simplicity, document security implications.

### Phase 5: Update Template Tag Processing

**File:** `internal/runner/template.go`

#### 5.1 Modify ExpandTemplates signature

```go
// Before
func ExpandTemplates[T any](in *T, variables map[string]string) error

// After
func ExpandTemplates[T any](in *T, ctx InterpolationContext) error
```

#### 5.2 Update field expansion

Change `Expand()` calls to `Interpolate()` calls with the new context.

#### 5.3 Support `map[string]any` for args

Current code only handles `map[string]string`. Need to support nested maps for terraform args.

### Phase 6: Validation

#### 6.1 Validate for_each structure

- Must be a map (not array or scalar)
- Keys must be valid identifiers (for use in expanded IDs)
- Values can be any structure

#### 6.2 Validate expanded IDs are unique

After expansion, check no duplicate collector or step IDs.

#### 6.3 Validate cross-references

After expansion, verify all step `collector` references point to existing collector IDs.

### Phase 7: Testing

#### 7.1 Unit tests for Interpolate

```go
func TestInterpolate(t *testing.T) {
    tests := []struct {
        name     string
        template string
        ctx      InterpolationContext
        want     string
        wantErr  bool
    }{
        {
            name:     "simple env",
            template: "home is ${env.HOME}",
            ctx:      InterpolationContext{Env: map[string]string{"HOME": "/home/user"}},
            want:     "home is /home/user",
        },
        {
            name:     "nested each value",
            template: "context: ${each.value.context}",
            ctx:      InterpolationContext{Each: &EachContext{Key: "dev", Value: map[string]any{"context": "dev-cluster"}}},
            want:     "context: dev-cluster",
        },
        // ... more cases
    }
}
```

#### 7.2 Unit tests for expansion

- Single collector without for_each
- Collector with for_each (2-3 iterations)
- Step with for_each referencing expanded collectors
- for_each referencing spec variables

#### 7.3 Integration tests

- Full job parse with for_each collectors and steps
- Verify expanded pipeline has correct collectors and steps
- Verify results have correct keys

### Phase 8: Documentation

#### 8.1 Update job spec documentation

Document:
- `spec.variables` structure and usage
- `for_each` syntax for collectors and steps
- Variable namespaces (`env`, `var`, `each`)
- Expansion behavior and result naming

#### 8.2 Add examples

Create example job files:
- `examples/multi-environment.yaml`
- `examples/for-each-basic.yaml`

## Migration Considerations

### Breaking Changes

The move from `os.Expand` to explicit namespaces is a breaking change:

| Before | After |
|--------|-------|
| `${HOME}` | `${env.HOME}` |
| `${MY_VAR}` | `${env.MY_VAR}` |

### Migration Path

Option A: Support both syntaxes temporarily
- Bare `${VAR}` falls back to env lookup with deprecation warning
- Remove in next major version

Option B: Clean break
- Require explicit namespaces immediately
- Document migration in release notes

Recommend Option B for cleaner implementation.

## Open Questions

1. **Should `var` support self-references?**
   ```yaml
   variables:
     base: us-east-1
     full: ${var.base}-cluster  # Reference another variable?
   ```
   Recommendation: No for V1, adds complexity with ordering/cycles.

2. **Error handling for missing each.value fields?**
   - Option: Use expr's `?.` and `??` operators for optional access
   - `${each.value.optional_field ?? "default"}`

3. **Should for_each support arrays?**
   ```yaml
   for_each:
     - dev
     - live
   ```
   Recommendation: No for V1. Map-of-maps is clearer and keys are useful.

## File Summary

| File | Changes |
|------|---------|
| `go.mod` | Add expr dependency |
| `api/v1/types.go` | Add Variables, ForEach fields |
| `internal/runner/template.go` | Replace os.Expand with expr-based interpolation |
| `internal/runner/expand.go` | New file: expansion logic |
| `internal/runner/job.go` | Integrate expansion into parse pipeline |
| `internal/runner/*_test.go` | Add tests |
| `website/src/content/docs/` | Update documentation |
| `examples/` | Add example files |
