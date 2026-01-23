# CollectJob YAML Specification

## Overview

A `CollectJob` is a YAML document that defines a collection job for gathering infrastructure resources. It supports multiple collector types:
- **Terraform**: Uses Terraform providers through the `tf-data-client` library
- **HTTP**: Makes HTTP requests to REST APIs

### Output Behavior

Results are always written as **one file per step**, with filenames following the pattern `{step-id}.{extension}` (e.g., `deployments.json`). Each result includes:
- `id`: The step identifier that produced this result
- `data`: The actual result data from the data source

When writing to stdout, each result is written as a separate line. When writing to the filesystem, each result is written to its own file.

## Schema

```yaml
kind: CollectJob
metadata:
  name: string          # Required: Pipeline name
  description?: string  # Optional: Pipeline description
spec:
  collectors:           # Required: List of collector definitions
    - id: string        # Required: Unique collector identifier
      # One of the following collector types:
      terraform:
        provider: string    # Required: Provider name (e.g., "hashicorp/kubernetes")
        version?: string    # Optional: Provider version (e.g., "v2.32.0")
        args: object        # Required: Provider configuration arguments
      http:
        base_url: string    # Required: Base URL for HTTP requests
        headers?: object    # Optional: Default headers for all requests
        timeout?: duration  # Optional: Request timeout (default: 30s)
        insecure?: boolean  # Optional: Skip TLS verification (default: false)
        auth?:              # Optional: Authentication configuration
          basic?:           # Optional: Basic authentication
            username?: string
            password?: string
            encoded?: string  # Base64-encoded credentials
  steps:                # Required: List of collection steps
    - id: string        # Required: Unique step identifier
      collector: string # Required: Collector ID to use
      # One of the following step types:
      terraform_datasource:
        name: string        # Required: Terraform data source name
        args: object        # Required: Data source arguments
      http_get:
        path: string        # Required: Request path (appended to base_url)
        headers?: object    # Optional: Request-specific headers
        params?: object     # Optional: Query parameters
        response_type?: string  # Optional: "json" (default) or "raw"
  output?:              # Optional: Output configuration
    encoding?:           # Optional: Encoding format configuration
      json?:             # Optional: JSON encoding options
        indent?: string  # Optional: Indentation (empty = compact, "  " = 2 spaces, "\t" = tabs)
    sink?:               # Optional: Output destination configuration
      stdout?: {}        # Optional: Write to stdout (one result per line)
      filesystem?:       # Optional: Write to filesystem (one file per step)
        path?: string    # Optional: Directory path (default: current directory)
        prefix?: string  # Optional: Prefix for output directory (supports $JOB_NAME, $JOB_DATE_RFC3339)
```

## Field Descriptions

### Top-Level Fields

#### `kind` (required)
- **Type**: `string`
- **Value**: Must be exactly `"CollectJob"`
- **Description**: Identifies the document type

#### `metadata` (required)
- **Type**: `object`
- **Description**: Metadata about the pipeline

##### `metadata.name` (required)
- **Type**: `string`
- **Description**: Unique name for the pipeline
- **Constraints**: Must be a valid identifier (alphanumeric, hyphens, underscores)

##### `metadata.description` (optional)
- **Type**: `string`
- **Description**: Human-readable description of the pipeline

#### `spec` (required)
- **Type**: `object`
- **Description**: Pipeline specification

##### `spec.collectors` (required)
- **Type**: `array`
- **Description**: List of collector definitions
- **Constraints**: Must contain at least one collector
- **Uniqueness**: Each collector must have a unique `id`

##### `spec.steps` (required)
- **Type**: `array`
- **Description**: List of collection steps
- **Constraints**: Must contain at least one step
- **Uniqueness**: Each step must have a unique `id`

##### `spec.output` (optional)
- **Type**: `object`
- **Description**: Configures how results are written
- **Default**: If not specified, results are written to stdout as compact JSON (one result per line)
- **Behavior**: Results are always written as one file per step, with filenames `{step-id}.{extension}` (e.g., `deployments.json`)

### Output Specification

#### `spec.output.encoding` (optional)
- **Type**: `object`
- **Description**: Configures the output format
- **Default**: JSON with compact output (no indentation)
- **Note**: Only one encoding type should be specified

##### `spec.output.encoding.json` (optional)
- **Type**: `object`
- **Description**: JSON encoding configuration

##### `spec.output.encoding.json.indent` (optional)
- **Type**: `string`
- **Description**: Indentation string for JSON output
- **Values**:
  - Empty string (`""`): Compact JSON (no indentation) - **default**
  - `"  "`: Two spaces per indentation level
  - `"\t"`: Tab character per indentation level
  - Any other string: Custom indentation
- **Examples**:
  - `indent: ""` - Compact JSON
  - `indent: "  "` - Pretty-printed with 2 spaces
  - `indent: "\t"` - Pretty-printed with tabs

#### `spec.output.sink` (optional)
- **Type**: `object`
- **Description**: Configures where output is written
- **Default**: If not specified, results are written to stdout
- **Note**: Only one sink type should be specified
- **Behavior**: All sinks write one file per step. For stdout, each result is written as a separate line.

##### `spec.output.sink.stdout` (optional)
- **Type**: `object`
- **Description**: Write output to standard output
- **Note**: Currently has no configuration options (empty object `{}`)
- **Format**: Each step's result is written as a separate line. Results include an `id` field identifying the step.

##### `spec.output.sink.filesystem` (optional)
- **Type**: `object`
- **Description**: Write output to files on the local filesystem
- **File Naming**: Each step's output is written to a file named `{step-id}.{extension}` (e.g., `deployments.json`)
- **Location**: Files are written to `{path}/{prefix}/` if both are specified, or just `{path}/` if only path is specified

##### `spec.output.sink.filesystem.path` (optional)
- **Type**: `string`
- **Description**: Directory path where output files will be written
- **Default**: Current working directory
- **Note**: Directory will be created if it doesn't exist

##### `spec.output.sink.filesystem.prefix` (optional)
- **Type**: `string`
- **Description**: Prefix prepended to the path, useful for organizing outputs by job name and date
- **Variables**: Supports variable substitution:
  - `$JOB_NAME`: Replaced with the job's `metadata.name`
  - `$JOB_DATE_RFC3339`: Replaced with current UTC time in RFC3339 format (e.g., `2026-01-19T08:18:15Z`)
- **Examples**:
  - `prefix: $JOB_NAME/$JOB_DATE_RFC3339` → `test/2026-01-19T08:18:15Z/`
  - `prefix: outputs` → `outputs/`

### Collector Specification

#### `collectors[].id` (required)
- **Type**: `string`
- **Description**: Unique identifier for the collector
- **Constraints**: Must be unique within the pipeline
- **Usage**: Referenced by steps in the `collector` field

#### `collectors[].terraform` (required)
- **Type**: `object`
- **Description**: Terraform provider configuration

##### `collectors[].terraform.provider` (required)
- **Type**: `string`
- **Description**: Provider name in the format `namespace/name`
- **Examples**:
  - `hashicorp/kubernetes`
  - `hashicorp/aws`
  - `hashicorp/azurerm`

##### `collectors[].terraform.version` (optional)
- **Type**: `string`
- **Description**: Provider version constraint
- **Examples**: `v2.32.0`, `~> 2.0`, `>= 2.0.0`
- **Default**: Latest available version

##### `collectors[].terraform.args` (required)
- **Type**: `object`
- **Description**: Provider-specific configuration arguments
- **Note**: Arguments vary by provider. Refer to provider documentation.

#### `collectors[].http` (optional)
- **Type**: `object`
- **Description**: HTTP collector configuration for REST API requests
- **Note**: Mutually exclusive with `terraform`

##### `collectors[].http.base_url` (required)
- **Type**: `string`
- **Description**: Base URL for all HTTP requests (must use http or https scheme)
- **Examples**: `https://api.example.com`, `http://localhost:8080/api/v1`

##### `collectors[].http.headers` (optional)
- **Type**: `object`
- **Description**: Default headers to include in all requests
- **Default Headers**: `User-Agent: infracollect/0.1.0`, `Accept: application/json`, `Accept-Encoding: gzip`
- **Note**: Custom headers override defaults

##### `collectors[].http.timeout` (optional)
- **Type**: `duration`
- **Description**: Request timeout
- **Default**: `30s`
- **Examples**: `10s`, `1m`, `500ms`

##### `collectors[].http.insecure` (optional)
- **Type**: `boolean`
- **Description**: Skip TLS certificate verification
- **Default**: `false`
- **Warning**: Only use for development/testing

##### `collectors[].http.auth` (optional)
- **Type**: `object`
- **Description**: Authentication configuration

##### `collectors[].http.auth.basic` (optional)
- **Type**: `object`
- **Description**: HTTP Basic authentication
- **Fields**:
  - `username`: Username for authentication
  - `password`: Password for authentication
  - `encoded`: Pre-encoded Base64 credentials (alternative to username/password)

### Step Specification

#### `steps[].id` (required)
- **Type**: `string`
- **Description**: Unique identifier for the step
- **Constraints**: Must be unique within the pipeline

#### `steps[].collector` (required)
- **Type**: `string`
- **Description**: ID of the collector to use for this step
- **Constraints**: Must reference a collector ID defined in `spec.collectors` of the matching type

#### `steps[].terraform_datasource` (optional)
- **Type**: `object`
- **Description**: Terraform data source configuration

##### `steps[].terraform_datasource.name` (required)
- **Type**: `string`
- **Description**: Terraform data source name
- **Examples**:
  - `kubernetes_resources`
  - `aws_instances`
  - `azurerm_resources`

##### `steps[].terraform_datasource.args` (required)
- **Type**: `object`
- **Description**: Data source-specific arguments
- **Note**: Arguments vary by data source. Refer to provider documentation.

#### `steps[].http_get` (optional)
- **Type**: `object`
- **Description**: HTTP GET request step configuration
- **Note**: Mutually exclusive with `terraform_datasource`

##### `steps[].http_get.path` (required)
- **Type**: `string`
- **Description**: Request path appended to the collector's base_url
- **Examples**: `/users`, `/api/v1/resources`, `/items?page=1`

##### `steps[].http_get.headers` (optional)
- **Type**: `object`
- **Description**: Additional headers for this specific request
- **Note**: These headers are merged with collector-level headers (request headers take precedence)

##### `steps[].http_get.params` (optional)
- **Type**: `object`
- **Description**: Query parameters to append to the request URL
- **Note**: Parameters are URL-encoded automatically

##### `steps[].http_get.response_type` (optional)
- **Type**: `string`
- **Description**: How to parse the response body
- **Values**:
  - `json` (default): Parse response as JSON
  - `raw`: Return response body as a string
- **Note**: Gzip-encoded responses are automatically decompressed

## Validation Rules

1. **Kind Validation**: `kind` must be `"CollectJob"`
2. **Metadata Validation**: `metadata.name` is required and must be a valid identifier
3. **Collector Validation**:
   - At least one collector must be defined
   - Each collector must have a unique `id`
   - Each collector must have exactly one of: `terraform` or `http`
   - Terraform collectors must have `provider` and `args`
   - HTTP collectors must have `base_url`
4. **Step Validation**:
   - At least one step must be defined
   - Each step must have a unique `id`
   - Each step must have exactly one of: `terraform_datasource` or `http_get`
   - Each step must reference a valid collector ID of the matching type
5. **Reference Validation**: All collector references in steps must exist and be of compatible type
6. **Output Validation**:
   - If `output.encoding` is specified, exactly one encoding type should be set
   - If `output.sink` is specified, exactly one sink type should be set

## Examples

### Kubernetes Example

```yaml
kind: CollectJob
metadata:
  name: k8s-deployments
  description: Collect Kubernetes deployments and pods
spec:
  collectors:
    - id: kind
      terraform:
        provider: hashicorp/kubernetes
        version: v2.32.0
        args:
          config_path: ./kubeconfig
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
    - id: pods
      collector: kind
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: v1
          kind: Pod
          namespace: kube-system
```

### Multi-Collector Example

```yaml
kind: CollectJob
metadata:
  name: multi-cloud-inventory
  description: Collect resources from multiple cloud providers
spec:
  collectors:
    - id: aws-us-east-1
      terraform:
        provider: hashicorp/aws
        version: ~> 5.0
        args:
          region: us-east-1
          access_key: ${AWS_ACCESS_KEY_ID}
          secret_key: ${AWS_SECRET_ACCESS_KEY}
    - id: aws-us-west-2
      terraform:
        provider: hashicorp/aws
        version: ~> 5.0
        args:
          region: us-west-2
          access_key: ${AWS_ACCESS_KEY_ID}
          secret_key: ${AWS_SECRET_ACCESS_KEY}
    - id: azure
      terraform:
        provider: hashicorp/azurerm
        version: ~> 3.0
        args:
          subscription_id: ${AZURE_SUBSCRIPTION_ID}
          client_id: ${AZURE_CLIENT_ID}
          client_secret: ${AZURE_CLIENT_SECRET}
          tenant_id: ${AZURE_TENANT_ID}
  steps:
    - id: aws-east-instances
      collector: aws-us-east-1
      terraform_datasource:
        name: aws_instances
        args:
          filters:
            - name: instance-state-name
              values: [running]
    - id: aws-west-instances
      collector: aws-us-west-2
      terraform_datasource:
        name: aws_instances
        args:
          filters:
            - name: instance-state-name
              values: [running]
    - id: azure-vms
      collector: azure
      terraform_datasource:
        name: azurerm_virtual_machines
        args:
          resource_group_name: production
```

### AWS Example

```yaml
kind: CollectJob
metadata:
  name: aws-ec2-inventory
spec:
  collectors:
    - id: aws-prod
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1
  steps:
    - id: ec2-instances
      collector: aws-prod
      terraform_datasource:
        name: aws_instances
        args:
          filters:
            - name: tag:Environment
              values: [production]
```

### HTTP API Example

```yaml
kind: CollectJob
metadata:
  name: api-collection
  description: Collect data from REST APIs
spec:
  collectors:
    - id: jsonplaceholder
      http:
        base_url: https://jsonplaceholder.typicode.com
    - id: internal-api
      http:
        base_url: https://api.internal.example.com
        timeout: 60s
        headers:
          X-API-Version: "2024-01"
        auth:
          basic:
            username: ${API_USERNAME}
            password: ${API_PASSWORD}
  steps:
    - id: users
      collector: jsonplaceholder
      http_get:
        path: /users
    - id: posts
      collector: jsonplaceholder
      http_get:
        path: /posts
        params:
          userId: "1"
    - id: resources
      collector: internal-api
      http_get:
        path: /api/v1/resources
        headers:
          Accept: application/json
        response_type: json
```

### Mixed Collectors Example

```yaml
kind: CollectJob
metadata:
  name: hybrid-collection
  description: Collect from both Terraform providers and HTTP APIs
spec:
  collectors:
    - id: k8s
      terraform:
        provider: hashicorp/kubernetes
        args:
          config_path: ~/.kube/config
    - id: monitoring-api
      http:
        base_url: https://monitoring.example.com/api
        auth:
          basic:
            encoded: ${MONITORING_API_TOKEN}
  steps:
    - id: deployments
      collector: k8s
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: apps/v1
          kind: Deployment
    - id: alerts
      collector: monitoring-api
      http_get:
        path: /v1/alerts
        params:
          status: active
```

### Output Configuration Examples

#### Pretty-Printed JSON to Stdout

```yaml
kind: CollectJob
metadata:
  name: k8s-deployments
spec:
  collectors:
    - id: kind
      terraform:
        provider: hashicorp/kubernetes
        args:
          config_path: ./kubeconfig
  steps:
    - id: deployments
      collector: kind
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: apps/v1
          kind: Deployment
  output:
    encoding:
      json:
        indent: "  "
    sink:
      stdout: {}
```

#### Compact JSON to Folder

```yaml
kind: CollectJob
metadata:
  name: aws-inventory
spec:
  collectors:
    - id: aws-prod
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1
  steps:
    - id: ec2-instances
      collector: aws-prod
      terraform_datasource:
        name: aws_instances
        args: {}
  output:
    encoding:
      json:
        indent: ""
    sink:
      filesystem:
        path: ./output
```

#### Pretty-Printed JSON to Filesystem with Prefix

```yaml
kind: CollectJob
metadata:
  name: multi-cloud-inventory
spec:
  collectors:
    - id: aws
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1
    - id: azure
      terraform:
        provider: hashicorp/azurerm
        args:
          subscription_id: ${AZURE_SUBSCRIPTION_ID}
  steps:
    - id: aws-instances
      collector: aws
      terraform_datasource:
        name: aws_instances
        args: {}
    - id: azure-vms
      collector: azure
      terraform_datasource:
        name: azurerm_virtual_machines
        args: {}
  output:
    encoding:
      json:
        indent: "\t"
    sink:
      filesystem:
        path: ./output
        prefix: $JOB_NAME/$JOB_DATE_RFC3339
```

#### Default Output (Compact JSON to Stdout)

When `output` is not specified, the default behavior is compact JSON to stdout:

```yaml
kind: CollectJob
metadata:
  name: simple-collection
spec:
  collectors:
    - id: aws
      terraform:
        provider: hashicorp/aws
        args:
          region: us-east-1
  steps:
    - id: instances
      collector: aws
      terraform_datasource:
        name: aws_instances
        args: {}
  # output not specified - defaults to compact JSON to stdout
```

## Provider-Specific Notes

### Kubernetes Provider

Common arguments:
- `config_path`: Path to kubeconfig file
- `config_context`: Kubernetes context to use
- `host`: Kubernetes API server URL
- `token`: Bearer token for authentication

### AWS Provider

Common arguments:
- `region`: AWS region
- `access_key`: AWS access key ID
- `secret_key`: AWS secret access key
- `profile`: AWS profile name
- `shared_credentials_file`: Path to credentials file

### Azure Provider

Common arguments:
- `subscription_id`: Azure subscription ID
- `client_id`: Azure client ID
- `client_secret`: Azure client secret
- `tenant_id`: Azure tenant ID

## Environment Variables

Provider arguments can reference environment variables using `${VARIABLE_NAME}` syntax. The system will substitute these values at runtime.

## Best Practices

1. **Naming**: Use descriptive names for collectors and steps
2. **Organization**: Group related steps together
3. **Secrets**: Use environment variables for sensitive data
4. **Versioning**: Pin provider versions for reproducibility
5. **Documentation**: Add descriptions to complex pipelines
6. **Validation**: Validate pipelines before execution
7. **Output Format**: Use pretty-printed JSON (`indent: "  "`) for human-readable output, compact JSON for machine processing
8. **Output Destination**: Use `filesystem` for local development and debugging, `stdout` for piping to other tools or streaming results
9. **File Organization**: Use the `prefix` field with `$JOB_NAME` and `$JOB_DATE_RFC3339` variables to organize outputs by job and timestamp
10. **Result Structure**: Each result includes an `id` field identifying the step that produced it, along with the `data` field containing the actual result data
