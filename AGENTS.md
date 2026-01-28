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
func TestParseCollectJob(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {
            name: "valid job",
            input: `kind: CollectJob
metadata:
  name: test
spec:
  collectors: []
  steps: []`,
            wantErr: false,
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := runner.ParseCollectJob([]byte(tt.input))
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseCollectJob() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // ... assertions
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
// It parses CollectJob YAML files and creates executable pipelines.
package runner
```

### Function Documentation

- Document all exported functions
- Use complete sentences
- Document parameters and return values
- Include examples for complex functions

```go
// ParseCollectJob parses a YAML or JSON job file and validates it against the JSON Schema
// generated from the v1.CollectJob struct. It returns a validated CollectJob struct or an error
// if parsing or validation fails.
func ParseCollectJob(data []byte) (v1.CollectJob, error) {
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

```go
// Pipeline manages collectors and steps
type Pipeline struct {
    name       string
    collectors map[string]Collector
    steps      map[string]Step
}

func (p *Pipeline) GetCollector(id string) (Collector, bool) {
    collector, ok := p.collectors[id]
    return collector, ok
}

func (p *Pipeline) Run(ctx context.Context) (map[string]Result, error) {
    results := make(map[string]Result)
    for id, step := range p.steps {
        result, err := step.Resolve(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve step '%s': %w", id, err)
        }
        results[id] = result
    }
    return results, nil
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

1. **Define interfaces first** in `pkg/engine/`
2. **Implement in appropriate package** (`internal/integrations/` for collectors, etc.)
3. **Add tests** alongside implementation
4. **Update documentation** (docs/ and code comments)
5. **Add examples** if the feature is user-facing (e.g., job.yaml)
6. **Update AGENTS.md** if patterns change

## Questions?

When in doubt:

- Follow Go standard library patterns
- Check existing code in the project for consistency
- Refer to `Effective Go` and `Go Code Review Comments`
- Ask for clarification rather than guessing
