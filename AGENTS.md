# AI Agent Guidelines for infracollect

This document provides guidelines and conventions for AI agents working on the infracollect codebase.

## Backlog Management

**IMPORTANT**: Before starting any task, check `BACKLOG.md` to see if the work relates to a tracked item.

### When Starting Work

1. Read `BACKLOG.md` to understand current priorities
2. If the task matches a backlog item, update its status to `[~]` (In Progress)
3. Work on tasks in priority order (P0 > P1 > P2 > P3) unless directed otherwise

### When Completing Work

1. Mark the backlog item as `[x]` (Done)
2. Move the completed item to the "Done" section with the completion date
3. Format: `[x] **Task Title** - Description (completed YYYY-MM-DD)`

### Adding New Items

When new work is identified during development:

1. Add it to the appropriate priority section
2. Use the format: `[ ] **Task Title** - Short description`

## Documentation

After working on a feature, look at the existing documentation in `website/src/content/docs/` and update it as needed.
If this is a new feature, add it to the appropriate section of the documentation following the Diataxis Framework.

## Code Style

### Go Conventions

- Follow standard Go formatting (`gofmt` / `goimports`)
- Use `golangci-lint` for linting (when configured)
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines

### Comments

- Only use comments for complex or non-obvious code
- Don't use comments to explain the obvious

### Naming Conventions

- **Packages**: Lowercase, single word, no underscores
- **Interfaces**: End with `-er` when appropriate (e.g., `Executor`, `Parser`)
- **Types**: PascalCase
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase
- **Constants**: PascalCase or UPPER_SNAKE_CASE for exported constants

## Interface Design Patterns

### Interface Naming

- Use descriptive names that indicate purpose
- End with `-er` suffix when appropriate: `Executor`, `Parser`, `Handler`
- For managers/registries: `Manager`, `Registry`, `Factory`

### Interface Methods

- Keep interfaces small and focused (Interface Segregation Principle)
- Methods should have clear, single responsibilities
- Use context.Context for cancellation and timeouts
- Return errors, never panic in interface methods

### Example Interface

```go
// Good: Focused, clear purpose
type Collector interface {
    Named
    Closer
    Start(context.Context) error
}

// Bad: Too many responsibilities
type Collector interface {
    // ... 20 methods doing different things
}
```

## Error Handling

### Error Wrapping

- Always wrap errors with context using `fmt.Errorf()` with `%w` verb
- Add context at each level of the call stack
- Use `errors.Is()` and `errors.As()` for error checking

### Example

```go
// Good
func (c *Collector) Start(ctx context.Context) error {
    provider, err := c.client.CreateProvider(ctx, c.providerConfig)
    if err != nil {
        return fmt.Errorf("failed to create provider: %w", err)
    }
    // ...
}

// Bad
func (c *Collector) Start(ctx context.Context) error {
    provider, err := c.client.CreateProvider(ctx, c.providerConfig)
    if err != nil {
        return err  // Lost context
    }
    // ...
}
```

### Error Types

- Define custom error types for specific error conditions
- Use sentinel errors for common cases
- Document when errors can occur

```go
// Custom error type
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error in field %s: %s", e.Field, e.Message)
}

// Sentinel error
var ErrCollectorNotFound = errors.New("collector not found")
```

## Testing Guidelines

### Test Structure

- Use table-driven tests for multiple test cases
- Test files: `*_test.go` in the same package
- Test functions: `TestXxx` for unit tests, `TestXxx_Scenario` for specific scenarios
- Always use stretchr/testify/assert and stretchr/testify/require methods for assertions

### Assertions

- Use `assert.ErrorContains(t, err, "substring")` instead of `assert.Contains(t, err.Error(), "substring")`
- Use `require.NoError(t, err)` for errors that should halt the test
- Use `assert.NoError(t, err)` for errors that should be reported but not halt

### Test Naming

```go
func TestSubprocessExecutor_InitProvider(t *testing.T) {
    // ...
}

func TestSubprocessExecutor_InitProvider_InvalidProvider(t *testing.T) {
    // ...
}
```

### Test Organization

- Test public interfaces thoroughly
- Test error cases
- Use test helpers for common setup
- Mock external dependencies

### Example Test

```go
func TestParseJobTemplate(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr string // substring; empty means no error
    }{
        {
            name: "valid job",
            input: `
job {
  name = "test"
}

step "static" "greeting" {
  value = "hello"
}`,
            wantErr: "",
        },
        // ... more test cases
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            _, diags := runner.ParseJobTemplate([]byte(tc.input), "test.hcl")
            if tc.wantErr == "" {
                require.False(t, diags.HasErrors(), "diags: %s", diags.Error())
                return
            }
            require.True(t, diags.HasErrors())
            assert.Contains(t, diags.Error(), tc.wantErr)
        })
    }
}
```

### Test context

Always use `t.Context()` as the context for tests.

## Documentation Standards

### Package Documentation

- Every package should have a package comment
- Explain the package's purpose and usage

```go
// Package runner provides job parsing and pipeline orchestration.
// It parses HCL job templates, builds a resolvable DAG, and executes
// collectors and steps in topological order.
package runner
```

### Function Documentation

- Document all exported functions
- Use complete sentences
- Document parameters and return values
- Include examples for complex functions

```go
// ParseJobTemplate parses HCL bytes into a JobTemplate. The filename is used
// only for diagnostic source ranges. Structural errors (unknown attributes,
// wrong labels, duplicate IDs) are returned as hcl.Diagnostics with source
// ranges pointing at the offending token.
func ParseJobTemplate(data []byte, filename string) (*JobTemplate, hcl.Diagnostics) {
    // ...
}
```

### Type Documentation

- Document exported types and their purpose
- Document struct fields when not obvious

```go
// Pipeline represents a collection pipeline with collectors and steps.
// It manages the lifecycle of collectors and executes steps to collect data.
type Pipeline struct {
    name       string
    collectors map[string]Collector
    steps      map[string]Step
}
```

## Implementation Patterns

### Collector Implementation

```go
// 1. Implement the interface
type Collector struct {
    providerConfig tfclient.ProviderConfig
    provider       *tfclient.Provider
    args           map[string]any
    client         *tfclient.Client
}

func (c *Collector) Name() string {
    return fmt.Sprintf("terraform(%s)", c.providerConfig.String())
}

func (c *Collector) Start(ctx context.Context) error {
    provider, err := c.client.CreateProvider(ctx, c.providerConfig)
    if err != nil {
        return fmt.Errorf("failed to create provider: %w", err)
    }
    provider.Configure(ctx, c.args)
    c.provider = provider
    return nil
}

// ... other methods
```

### Factory Pattern

```go
// NewCollector creates a terraform collector instance
func NewCollector(client *tfclient.Client, cfg Config) (engine.Collector, error) {
    provider, err := tfaddr.ParseProviderSource(cfg.Provider)
    if err != nil {
        return nil, fmt.Errorf("failed to parse provider source '%s': %w", cfg.Provider, err)
    }

    version := strings.TrimPrefix(cfg.Version, "v")

    return &Collector{
        providerConfig: tfclient.ProviderConfig{
            Namespace: provider.Namespace,
            Name:      provider.Type,
            Version:   version,
        },
        args:   cfg.Args,
        client: client,
    }, nil
}
```

### Pipeline Pattern

Pipeline owns the DAG and the per-node metadata (decoded `hcl.Body`, extracted
references, `for_each` expression). Execution lives in `runner.Run`, which
walks the DAG in topological order and does the second-pass gohcl decode
against a per-node `hcl.EvalContext`.

```go
// Pipeline holds the resolvable DAG for a job template.
type Pipeline struct {
    dag  *DirectedAcyclicGraph
    meta map[string]*NodeMeta // keyed by Node.Key()
}

func (p *Pipeline) Dag() *DirectedAcyclicGraph { return p.dag }

func (p *Pipeline) Meta(n Node) (*NodeMeta, bool) {
    m, ok := p.meta[n.Key()]
    return m, ok
}

// BuildPipeline decodes labels, extracts references via expr.Variables(),
// and wires DAG edges. No integration factory is called here.
func BuildPipeline(logger *zap.Logger, tmpl *JobTemplate, registry *engine.Registry) (*Pipeline, hcl.Diagnostics) {
    // ... walk tmpl.Collectors + tmpl.Steps, call ReferencesInBody, AddEdgeUnchecked ...
}
```

## Logging

### Structured Logging

- Use `zap` logger from context
- Include relevant context in log messages
- Use appropriate log levels:
  - `Debug`: Detailed information for debugging
  - `Info`: General informational messages
  - `Warn`: Warning messages for potentially problematic situations
  - `Error`: Error messages for failures
  - `Fatal`: Critical errors that cause program termination

### Example

```go
func (r *Runner) Run(ctx context.Context) (map[string]engine.Result, error) {
    logger := r.logger

    logger.Info("starting collectors")
    for _, collector := range r.pipeline.Collectors() {
        if err := collector.Start(ctx); err != nil {
            return nil, fmt.Errorf("failed to start collector: %w", err)
        }
    }

    logger.Info("running pipeline steps")
    results, err := r.pipeline.Run(ctx)
    if err != nil {
        logger.Error("pipeline execution failed", zap.Error(err))
        return nil, fmt.Errorf("failed to run pipeline: %w", err)
    }

    return results, nil
}
```

## Context Usage

- Always accept `context.Context` as the first parameter in functions that can be cancelled or timed out
- Pass context through the call chain
- Use context for cancellation and timeouts
- Don't store contexts in structs (pass them as parameters)

```go
// Good
func (c *Collector) Initialize(ctx context.Context) error {
    return c.executor.InitProvider(ctx, ...)
}

// Bad
type Collector struct {
    ctx context.Context  // Don't store context
}
```

## Concurrency

- Use `sync.RWMutex` for read-heavy concurrent access
- Use `sync.Mutex` for write-heavy or mixed access
- Document which methods are safe for concurrent use
- Use channels for communication between goroutines
- Use `context.Context` for cancellation

## Resource Management

- Always close resources (files, connections, executors)
- Use `defer` for cleanup
- Implement `Close()` methods for types that need cleanup
- Document cleanup requirements

```go
func (c *Collector) Close() error {
    if c.executor != nil {
        return c.executor.Close()
    }
    return nil
}
```

## Common Pitfalls to Avoid

1. **Don't ignore errors**: Always handle errors appropriately
2. **Don't use `_` for errors**: At least log ignored errors
3. **Don't panic**: Return errors instead
4. **Don't store contexts**: Pass them as parameters
5. **Don't create interfaces prematurely**: Start with concrete types, extract interfaces when needed
6. **Don't use global state**: Pass dependencies explicitly
7. **Don't mix concerns**: Keep functions focused on a single responsibility

## When Adding New Features

1. **Define interfaces first** in `internal/engine/`
2. **Implement in appropriate package** (`internal/integrations/` for collectors, etc.)
3. **Add tests** alongside implementation
4. **Update documentation** (website/src/content/docs/ and code comments)
5. **Add examples** if the feature is user-facing (an HCL job snippet in docs)
6. **Update CLAUDE.md** if patterns change

## Adding New Step Types or Collectors

Each integration owns its own HCL config struct with `hcl:"..."` tags. There
is no central `apis/v1` schema — config types live next to the factory that
consumes them.

When adding a new step type (like `exec`, `static`) or collector (like
`terraform`, `http`):

1. **Define the config struct** in the integration package (e.g.
   `internal/integrations/<name>/<name>.go` or
   `internal/engine/steps/<name>.go`) with `hcl:"..."` tags. Use nested
   labeled blocks for discriminated unions (e.g. `auth "basic" { ... }`).
2. **Implement the `engine.Collector` or `engine.Step` interface** in the
   same package.
3. **Register a factory** in `internal/engine/steps/register.go` (for steps)
   or the integration-specific register site (for collectors). The factory
   receives an `hcl.Body` plus the per-node `hcl.EvalContext` and calls
   `gohcl.DecodeBody` to populate the config struct — the runner's topo walk
   has already stamped predecessors into the context by that point.
4. **Surface the new kind to the registry** so `BuildPipeline`'s known-kinds
   gate accepts it — `registry.RegisterCollector` / `registry.RegisterStep`.
5. **Add tests**: factory decode happy/error paths, plus a runner-level test
   via the stub registry pattern in `internal/runner/run_test.go` if the step
   has cross-step reference or `for_each` behaviour worth exercising.
6. **Update reference docs** in `website/src/content/docs/reference/steps/`
   or `reference/collectors/` with the HCL block shape.

Note: `scripts/gen-docs.go` currently walks the removed `apis/v1` tree and is
parked as broken pending a rewrite that reads integration-local gohcl tags.
Do not rely on it to produce schema docs.

## Questions?

When in doubt:

- Follow Go standard library patterns
- Check existing code in the project for consistency
- Refer to `Effective Go` and `Go Code Review Comments`
- Ask for clarification rather than guessing
