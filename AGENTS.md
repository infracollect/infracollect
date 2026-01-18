# AI Agent Guidelines for infracollect

This document provides guidelines and conventions for AI agents working on the infracollect codebase.

## Code Style

### Go Conventions

- Follow standard Go formatting (`gofmt` / `goimports`)
- Use `golangci-lint` for linting (when configured)
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofumpt`-style formatting for stricter formatting

### Naming Conventions

- **Packages**: Lowercase, single word, no underscores
- **Interfaces**: End with `-er` when appropriate (e.g., `Executor`, `Parser`)
- **Types**: PascalCase
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase
- **Constants**: PascalCase or UPPER_SNAKE_CASE for exported constants

### File Organization

- One main type per file when possible
- Related types can be grouped in the same file
- Interface definitions in `pkg/engine/`
- Collector implementations in `pkg/collectors/`

## Project Structure

```
infracollect/
├── cmd/infracollect/          # CLI entry point
│   ├── main.go               # Main CLI application
│   ├── collect.go            # Collect command implementation
│   └── logging.go            # Logging setup
├── pkg/
│   ├── engine/               # Core interfaces and types
│   │   ├── collector.go      # Collector interface
│   │   ├── core.go           # Core interfaces (Named, Closer)
│   │   ├── pipeline.go       # Pipeline type and interface
│   │   ├── result.go         # Result type
│   │   └── step.go           # Step interface
│   ├── collectors/
│   │   └── terraform/        # Terraform collector implementation
│   │       ├── collector.go  # Collector implementation
│   │       └── steps.go      # Data source step implementation
│   └── runner/               # Pipeline creation and execution
│       ├── pipeline.go       # Pipeline factory
│       └── run.go            # Runner and job parsing
├── apis/v1/                  # API type definitions
│   ├── base.go               # Base types (Metadata)
│   └── job.go                # CollectJob types
├── docs/                     # Documentation
└── job.yaml                  # Example job file
```

### Package Guidelines

- **`pkg/engine/`**: Public interfaces and types that define the core abstractions
- **`pkg/collectors/`**: Collector implementations (currently terraform)
- **`pkg/runner/`**: Job parsing and pipeline orchestration
- **`apis/v1/`**: API type definitions (YAML/JSON schemas)
- **`cmd/`**: Application entry points

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
2. **Implement in appropriate package** (`pkg/collectors/` for collectors, etc.)
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
