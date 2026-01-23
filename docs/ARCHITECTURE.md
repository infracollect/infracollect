# infracollect Architecture

## System Overview

infracollect follows a pipeline-based architecture where YAML-defined collection jobs are parsed, validated, and executed to collect infrastructure resources. It supports multiple collector types:
- **Terraform collectors**: Use Terraform providers via the `tf-data-client` library
- **HTTP collectors**: Make HTTP requests to REST APIs

## Architecture Diagram

```mermaid
flowchart TD
    YAML[YAML Job Definition] --> Parser[Job Parser]
    Parser --> Validate[Validation]
    Validate --> Runner[Runner]
    Runner --> Pipeline[Pipeline]

    Pipeline --> Collectors[Collectors]
    Collectors --> TFC[Terraform Collectors]
    Collectors --> HC[HTTP Collectors]

    TFC --> TFClient[tf-data-client]
    TFClient --> Provider1[Terraform Provider 1]
    TFClient --> ProviderN[Terraform Provider N]

    HC --> HTTPClient[HTTP Client]
    HTTPClient --> API1[REST API 1]
    HTTPClient --> APIN[REST API N]

    Provider1 --> Results1[Resource Data]
    ProviderN --> Results2[Resource Data]
    API1 --> Results3[API Response]
    APIN --> Results4[API Response]

    Pipeline --> Steps[Steps]
    Steps --> TFStep[Terraform Steps]
    Steps --> HTTPStep[HTTP Steps]

    TFStep --> TFC
    HTTPStep --> HC

    Results1 --> Final[Results Map]
    Results2 --> Final
    Results3 --> Final
    Results4 --> Final
```

## Component Breakdown

### 1. Job Parser

**Location**: `pkg/runner/`

**Responsibilities**:
- Parse YAML files into `CollectJob` structs (defined in `apis/v1/`)
- Validate job structure and references using JSON Schema
- Ensure collector IDs referenced in steps exist
- Validate YAML schema against the `CollectJob` struct

**Key Functions**:
- `ParseCollectJob()`: Parses and validates YAML job files

### 2. Pipeline

**Location**: `pkg/engine/pipeline.go`

**Responsibilities**:
- Manage collectors and steps in a single pipeline
- Provide collector lookup by ID
- Execute all steps and collect results
- Maintain lifecycle of collectors and steps

**Key Methods**:
- `AddCollector()`: Add a collector to the pipeline
- `AddStep()`: Add a step to the pipeline
- `GetCollector()`: Retrieve a collector by ID
- `Run()`: Execute all steps and return results

### 3. Collectors

**Locations**:
- `pkg/collectors/terraform/` - Terraform provider collector
- `pkg/collectors/http/` - HTTP REST API collector

**Responsibilities**:
- Abstract data collection from various sources
- Initialize and configure connections
- Execute queries/requests
- Manage lifecycle (start/close)

**Interfaces**:
- `Collector` (in `pkg/engine/collector.go`)

#### Terraform Collector

**Responsibilities**:
- Wrap Terraform providers using `tf-data-client`
- Initialize and configure providers
- Execute data source queries

#### HTTP Collector

**Responsibilities**:
- Make HTTP requests to REST APIs
- Handle authentication (Basic auth)
- Manage headers, timeouts, and TLS settings
- Support gzip response decompression

### 4. Steps

**Locations**:
- `pkg/collectors/terraform/steps.go` - Terraform data source steps
- `pkg/collectors/http/steps.go` - HTTP request steps

**Responsibilities**:
- Represent a data collection operation
- Reference a collector
- Execute queries/requests through the collector
- Return results in standardized format

**Interfaces**:
- `Step` (in `pkg/engine/step.go`)

**Implementations**:
- `dataSourceStep`: Executes Terraform data sources through terraform collectors
- `getStep`: Executes HTTP GET requests through HTTP collectors

### 5. tf-data-client Integration

**Location**: External library `github.com/adrien-f/tf-data-client`

**Responsibilities**:
- Directly run Terraform providers as library components (not via CLI)
- Create and configure provider instances
- Execute data source queries
- Manage provider lifecycle

**Key Features**:
- No need for OpenTofu CLI or HCL generation
- Direct Go library integration with Terraform providers
- Handles provider initialization and configuration internally

## Data Flow

### 1. Job Definition → Parsing

```
YAML File → runner.ParseCollectJob() → CollectJob struct
```

### 2. Validation

```
CollectJob → JSON Schema Validation → Validated Job
```

### 3. Pipeline Creation

```
CollectJob → runner.createPipeline() → Pipeline with Collectors and Steps
→ terraform.NewCollector() → Collector instances
```

### 4. Collector Initialization

**Terraform Collectors**:
```
Collector.Start() → tf-data-client.CreateProvider() → Provider instance
→ Provider.Configure() → Provider configured
```

**HTTP Collectors**:
```
Collector.Start() → (no-op, HTTP client is ready)
```

### 5. Step Execution

**Terraform Steps**:
```
Pipeline.Run() → Step.Resolve() → Collector.ReadDataSource()
→ tf-data-client Provider → Resource Data
→ Result struct
```

**HTTP Steps**:
```
Pipeline.Run() → Step.Resolve() → Collector.Do(request)
→ HTTP Client → API Response
→ JSON/Raw parsing → Result struct
```

### 6. Result Collection

```
Step Results → Map[string]Result → Returned to Runner
```

### 7. Result Writing

```
Runner.WriteResults() → Encoder.EncodeResult() → Sink.Write()
→ Files written (one per step) or stdout output
```

The Runner handles all result writing:
- Encodes each result using the configured encoder
- Writes to the configured sink (stdout or filesystem)
- Each step's result is written as a separate file with filename `{step-id}.{extension}`
- Results include an `id` field identifying the step

## Multi-Collector Execution Model

### Isolation

Each collector operates independently:

**Terraform Collectors**:
- Each collector has its own provider instance managed by `tf-data-client`
- Providers are isolated at the library level
- No shared state between collectors

**HTTP Collectors**:
- Each collector has its own HTTP client instance
- Separate base URLs, headers, and authentication
- Connection pooling per collector

### Concurrent Execution

- Collectors are initialized sequentially (all started before steps run)
- Steps execute sequentially through the pipeline
- Each collector maintains its own provider instance
- Multiple steps can reference the same collector

### Step Execution

Steps reference collectors by ID:

```yaml
steps:
  # Terraform data source step
  - id: step1
    terraform_datasource:
      collector: k8s-collector  # References terraform collector
      name: kubernetes_resources
      args: {...}
  # HTTP GET step
  - id: step2
    http_get:
      collector: api-collector  # References HTTP collector
      path: /users
      response_type: json
```

## Interface Contracts

### Collector Interface

```go
type Collector interface {
    Named
    Closer
    Start(context.Context) error
}
```

**Implementations**:

`terraform.Collector`:
- `Start()`: Initializes and configures the Terraform provider via `tf-data-client`
- `ReadDataSource()`: Executes a data source query (used by terraform steps)
- `Close()`: Cleans up the provider instance

`http.Collector`:
- `Start()`: No-op (HTTP client is created in constructor)
- `Do()`: Executes an HTTP request (used by HTTP steps)
- `Close()`: No-op (HTTP client cleanup is automatic)

### Step Interface

```go
type Step interface {
    Named
    Resolve(ctx context.Context) (Result, error)
}
```

Steps execute data collection operations and return results.

**Implementations**:
- `terraform.NewDataSourceStep()`: Creates steps that execute Terraform data sources
- `http.NewGetStep()`: Creates steps that execute HTTP GET requests

### Result Type

```go
type Result struct {
    ID   string `json:"id"`
    Data any    `json:"data"`
}
```

Results contain:
- `ID`: The step identifier that produced this result
- `Data`: The collected data from the step's data source query

### Runner

**Location**: `pkg/runner/run.go`

**Responsibilities**:
- Orchestrate pipeline execution
- Manage collector lifecycle (start/close)
- Write results to configured sink
- Handle encoding and output formatting

**Key Methods**:
- `New()`: Creates a new Runner with pipeline, encoder, and sink
- `Run()`: Executes the pipeline and writes results
- `WriteResults()`: Encodes and writes results to the sink

**Output Behavior**:
- Always writes one file per step with filename `{step-id}.{extension}`
- For stdout: each result written as a separate line
- For filesystem: each result written to its own file in the configured directory

## Error Handling

- **Validation Errors**: Returned during job parsing/validation
- **Initialization Errors**: Returned when collectors fail to initialize
- **Execution Errors**: Returned when data source queries fail
- **Step Resolution Errors**: Returned when steps fail to resolve

All errors are wrapped with context and returned through the interface methods. Errors include relevant identifiers (collector IDs, step IDs) to aid debugging.

## Logging

- Structured logging using `zap`
- Log levels: debug, info, warn, error, fatal
- Context-aware logging throughout the pipeline
- Collector-specific log contexts

## Output System

The output system has been simplified to remove the Output abstraction layer. Results are now written directly by the Runner:

### Encoders

**Location**: `pkg/engine/encoders/`

**Responsibilities**:
- Encode results into specific formats (JSON, YAML, etc.)
- Provide file extensions for output files

**Interfaces**:
- `Encoder`: Encodes a single result to a reader

### Sinks

**Location**: `pkg/engine/sinks/`

**Responsibilities**:
- Write encoded data to destinations (stdout, filesystem)
- Handle file creation and directory management

**Interfaces**:
- `Sink`: Writes data to a destination with a given path

**Types**:
- `StreamSink`: Writes to an `io.Writer` (typically stdout)
- `FilesystemSink`: Writes to files on the local filesystem

### Writing Flow

1. Runner collects results from Pipeline.Run()
2. For each result:
   - Encoder encodes the result
   - Sink writes the encoded data with filename `{step-id}.{extension}`
3. Sink is closed after all results are written

## Future Architecture Considerations

- **Plugin System**: Load collectors and output handlers dynamically
- **Caching Layer**: Cache provider schemas and collected data
- **Scheduler**: Support scheduled pipeline execution
- **API Server**: REST API for pipeline management
- **Web UI**: Dashboard for viewing collected resources
