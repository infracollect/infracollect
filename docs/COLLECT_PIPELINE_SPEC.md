# CollectJob YAML Specification

## Overview

A `CollectJob` is a YAML document that defines a collection job for gathering infrastructure resources. It supports
multiple collector types:

- **Terraform**: Uses Terraform providers through the `tf-data-client` library
- **HTTP**: Makes HTTP requests to REST APIs
- **Static**: Loads data from local files or inline values (no collector required)

### Output Behavior

Results are written as **one file per step** with filenames `{step-id}.{extension}` (e.g., `deployments.json`). Each
result includes:

- `id`: The step identifier that produced this result
- `data`: The actual result data from the data source

**Without archive**: When writing to stdout, each result is a separate line; when writing to the filesystem or S3, each
result is its own file.

**With archive**: When `output.archive` is set, all step outputs are collected into a single tar archive (with optional
gzip or zstd compression) before being written to the sink. The sink receives one file (e.g.,
`my-job-20260124T120000Z.tar.gz`) containing `users.json`, `posts.json`, etc. Archive requires a filesystem or S3 sink;
stdout cannot be used with archive.

## Schema

```yaml
kind: CollectJob
metadata:
  name: string # Required: Pipeline name
  description?: string # Optional: Pipeline description
spec:
  collectors?: # Optional: List of collector definitions (required if using terraform or http steps)
    - id: string # Required: Unique collector identifier
      # One of the following collector types:
      terraform:
        provider: string # Required: Provider name (e.g., "hashicorp/kubernetes")
        version?: string # Optional: Provider version (e.g., "v2.32.0")
        args: object # Required: Provider configuration arguments
      http:
        base_url: string # Required: Base URL for HTTP requests
        headers?: object # Optional: Default headers for all requests
        timeout?: duration # Optional: Request timeout (default: 30s)
        insecure?: boolean # Optional: Skip TLS verification (default: false)
        auth?: # Optional: Authentication configuration
          basic?: # Optional: Basic authentication
            username?: string
            password?: string
            encoded?: string # Base64-encoded credentials
  steps: # Required: List of collection steps
    - id: string # Required: Unique step identifier
      collector?: string # Optional: Collector ID (required for terraform_datasource and http_get)
      # One of the following step types:
      terraform_datasource:
        name: string # Required: Terraform data source name
        args: object # Required: Data source arguments
      http_get:
        path: string # Required: Request path (appended to base_url)
        headers?: object # Optional: Request-specific headers
        params?: object # Optional: Query parameters
        response_type?: string # Optional: "json" (default) or "raw"
      static:
        filepath?: string # Optional: Relative path to a local file (mutually exclusive with value)
        value?: string # Optional: Inline value (mutually exclusive with filepath)
        parse_as?: string # Optional: "json" or "raw" (default: auto-detect for .json files)
  output?: # Optional: Output configuration
    encoding?: # Optional: Encoding format configuration
      json?: # Optional: JSON encoding options
        indent?: string # Optional: Indentation (empty = compact, "  " = 2 spaces, "\t" = tabs)
    archive?: # Optional: Bundle all step outputs into a single archive (requires filesystem or S3 sink)
      format: string # Required: "tar" (only supported format)
      compression?: string # Optional: "gzip" (default), "zstd", or "none"
      name?: string # Optional: Archive base name; extension auto-appended. Default: $JOB_NAME. Supports $JOB_NAME, $JOB_DATE_ISO8601, $JOB_DATE_RFC3339
    sink?: # Optional: Output destination configuration
      stdout?: {} # Optional: Write to stdout (one result per line)
      filesystem?: # Optional: Write to filesystem (one file per step, or one archive if output.archive is set)
        path?: string # Optional: Directory path (default: current directory)
        prefix?: string # Optional: Prefix for output directory (supports $JOB_NAME, $JOB_DATE_ISO8601, $JOB_DATE_RFC3339)
      s3?: # Optional: Write to S3-compatible storage (AWS S3, R2, MinIO)
        bucket: string # Required: S3 bucket name
        region?: string # Optional: AWS region
        endpoint?: string # Optional: Custom endpoint for S3-compatible services
        prefix?: string # Optional: Object key prefix (supports $JOB_NAME, $JOB_DATE_ISO8601, $JOB_DATE_RFC3339)
        force_path_style?: boolean # Optional: Use path-style addressing (for MinIO)
        credentials?: # Optional: Explicit credentials (uses SDK chain if not specified)
          access_key_id: string
          secret_access_key: string
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

##### `spec.collectors` (optional)

- **Type**: `array`
- **Description**: List of collector definitions
- **Constraints**: Required if any steps use `terraform_datasource` or `http_get`. Can be empty or omitted if all steps
  are `static`.
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
- **Behavior**: Results are always written as one file per step, with filenames `{step-id}.{extension}` (e.g.,
  `deployments.json`)

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

#### `spec.output.archive` (optional)

- **Type**: `object`
- **Description**: Bundles all step outputs into a single tar archive before writing to the sink. Each step's encoded
  result is added as a file in the archive (e.g., `users.json`, `posts.json`). The archive is written as one file to the
  underlying sink (filesystem or S3).
- **Requirement**: Archive cannot be used with stdout; use a filesystem or S3 sink.
- **Default**: If not specified, outputs are written as separate files (one per step).

##### `spec.output.archive.format` (required when archive is specified)

- **Type**: `string`
- **Description**: Archive format
- **Values**: `tar` (only supported format)
- **Example**: `format: tar`

##### `spec.output.archive.compression` (optional)

- **Type**: `string`
- **Description**: Compression algorithm for the archive
- **Values**:
  - `gzip` (default): Produces `.tar.gz`
  - `zstd`: Produces `.tar.zst`
  - `none`: Uncompressed `.tar`
- **Examples**: `compression: gzip`, `compression: zstd`

##### `spec.output.archive.name` (optional)

- **Type**: `string`
- **Description**: Base name for the archive file. The correct extension (`.tar.gz`, `.tar.zst`, or `.tar`) is appended
  automatically.
- **Default**: `$JOB_NAME`
- **Variables**: Supports variable substitution:
  - `$JOB_NAME`: The job's `metadata.name`
  - `$JOB_DATE_ISO8601`: Current UTC time in ISO8601 basic format (e.g., `20260124T120000Z`)
  - `$JOB_DATE_RFC3339`: Current UTC time in RFC3339 format (e.g., `2026-01-24T12:00:00Z`)
- **Examples**:
  - `name: $JOB_NAME` → `my-job.tar.gz`
  - `name: $JOB_NAME-$JOB_DATE_ISO8601` → `my-job-20260124T120000Z.tar.gz`

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
- **File Naming**: Without archive, each step's output is written to a file named `{step-id}.{extension}` (e.g.,
  `deployments.json`). With `output.archive`, the sink receives a single archive file (e.g.,
  `my-job-20260124T120000Z.tar.gz`) containing all step outputs.
- **Location**: Files are written to `{path}/{prefix}/` if both are specified, or just `{path}/` if only path is
  specified

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
  - `$JOB_DATE_ISO8601`: Replaced with current UTC time in ISO8601 basic format (e.g., `20260119T081815Z`) -
    **recommended**
  - `$JOB_DATE_RFC3339`: Replaced with current UTC time in RFC3339 format (e.g., `2026-01-19T08:18:15Z`)
- **Examples**:
  - `prefix: $JOB_NAME/$JOB_DATE_ISO8601` → `test/20260119T081815Z/`
  - `prefix: outputs` → `outputs/`

##### `spec.output.sink.s3` (optional)

- **Type**: `object`
- **Description**: Write output to S3-compatible object storage (AWS S3, Cloudflare R2, MinIO, etc.)
- **File Naming**: Without archive, each step's output is written as an object with key `{prefix}/{step-id}.{extension}`
  (e.g., `exports/deployments.json`). With `output.archive`, the sink receives a single archive object (e.g.,
  `{prefix}/my-job-20260124T120000Z.tar.gz`).
- **Credentials**: Uses AWS SDK credential chain by default (env vars, shared credentials file, IAM role). Explicit
  credentials can be provided via the `credentials` field.

##### `spec.output.sink.s3.bucket` (required)

- **Type**: `string`
- **Description**: S3 bucket name
- **Examples**: `my-bucket`, `infracollect-exports`

##### `spec.output.sink.s3.region` (optional)

- **Type**: `string`
- **Description**: AWS region for the bucket
- **Default**: Uses SDK defaults (AWS_REGION env var, config file, or auto-detection)
- **Examples**: `us-east-1`, `eu-west-1`

##### `spec.output.sink.s3.endpoint` (optional)

- **Type**: `string`
- **Description**: Custom endpoint URL for S3-compatible services
- **Note**: Required for Cloudflare R2, MinIO, and other non-AWS services
- **Examples**:
  - Cloudflare R2: `https://<ACCOUNT_ID>.r2.cloudflarestorage.com`
  - MinIO: `http://localhost:9000`

##### `spec.output.sink.s3.prefix` (optional)

- **Type**: `string`
- **Description**: Prefix prepended to object keys, useful for organizing outputs
- **Variables**: Supports variable substitution:
  - `$JOB_NAME`: Replaced with the job's `metadata.name`
  - `$JOB_DATE_ISO8601`: Replaced with current UTC time in ISO8601 basic format (e.g., `20260119T081815Z`) -
    **recommended**
  - `$JOB_DATE_RFC3339`: Replaced with current UTC time in RFC3339 format (e.g., `2026-01-19T08:18:15Z`)
- **Note**: `$JOB_DATE_ISO8601` is recommended for S3 keys because RFC3339 contains colons (`:`) which require URL
  encoding
- **Examples**:
  - `prefix: $JOB_NAME/$JOB_DATE_ISO8601` → `my-job/20260119T081815Z/`
  - `prefix: exports` → `exports/`

##### `spec.output.sink.s3.force_path_style` (optional)

- **Type**: `boolean`
- **Description**: Use path-style addressing instead of virtual-hosted-style
- **Default**: `false`
- **Note**: Required for MinIO and some S3-compatible services that don't support virtual-hosted-style URLs

##### `spec.output.sink.s3.credentials` (optional)

- **Type**: `object`
- **Description**: Explicit AWS credentials
- **Note**: If not specified, uses the AWS SDK credential chain (environment variables, shared credentials file, IAM
  role)

##### `spec.output.sink.s3.credentials.access_key_id` (required when credentials is specified)

- **Type**: `string`
- **Description**: AWS access key ID

##### `spec.output.sink.s3.credentials.secret_access_key` (required when credentials is specified)

- **Type**: `string`
- **Description**: AWS secret access key

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

#### `steps[].collector` (optional)

- **Type**: `string`
- **Description**: ID of the collector to use for this step
- **Constraints**: Required for `terraform_datasource` and `http_get` steps. Must reference a collector ID defined in
  `spec.collectors` of the matching type. Not used for `static` steps.

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

#### `steps[].static` (optional)

- **Type**: `object`
- **Description**: Static data step that loads data from a local file or inline value
- **Note**: Mutually exclusive with `terraform_datasource` and `http_get`. Does not require a collector.

##### `steps[].static.filepath` (optional)

- **Type**: `string`
- **Description**: Relative path to a local file to read
- **Constraints**: Must be a relative path within the working directory. Path traversal (e.g., `../`) is blocked for
  security.
- **Note**: Mutually exclusive with `value`. Files with `.json` extension are automatically parsed as JSON unless
  `parse_as: raw` is specified.
- **Examples**: `data/config.json`, `inventory.txt`, `templates/base.yaml`

##### `steps[].static.value` (optional)

- **Type**: `string`
- **Description**: Inline value to use as the step's data
- **Note**: Mutually exclusive with `filepath`. Useful for embedding small JSON objects or configuration directly in the
  job file.
- **Examples**: `{"key": "value"}`, `plain text content`

##### `steps[].static.parse_as` (optional)

- **Type**: `string`
- **Description**: How to parse the file or value content
- **Values**:
  - `json`: Parse content as JSON and return the parsed object
  - `raw`: Return content as a string without parsing
- **Default**: For files, auto-detects based on extension (`.json` files are parsed as JSON, others as raw). For inline
  values, defaults to `raw` unless explicitly set to `json`.

## Validation Rules

1. **Kind Validation**: `kind` must be `"CollectJob"`
2. **Metadata Validation**: `metadata.name` is required and must be a valid identifier
3. **Collector Validation**:
   - Collectors are optional (a job with only static steps needs no collectors)
   - Each collector must have a unique `id`
   - Each collector must have exactly one of: `terraform` or `http`
   - Terraform collectors must have `provider` and `args`
   - HTTP collectors must have `base_url`
4. **Step Validation**:
   - At least one step must be defined
   - Each step must have a unique `id`
   - Each step must have exactly one of: `terraform_datasource`, `http_get`, or `static`
   - Steps with `terraform_datasource` or `http_get` must have a `collector` reference
   - Steps with `static` must not have a `collector` reference
   - Static steps must have exactly one of: `filepath` or `value`
5. **Reference Validation**: All collector references in steps must exist and be of compatible type
6. **Output Validation**:
   - If `output.encoding` is specified, exactly one encoding type should be set
   - If `output.sink` is specified, exactly one sink type should be set
   - If `output.archive` is specified, `output.sink` must be filesystem or S3; stdout cannot be used with archive
   - If `output.archive` is specified, `archive.format` must be `tar`

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

### Static Step Example

```yaml
kind: CollectJob
metadata:
  name: static-data-collection
  description: Collect data from local files and inline values
spec:
  collectors: []
  steps:
    # Load a JSON file (auto-parsed)
    - id: config
      static:
        filepath: config/settings.json

    # Load a text file as raw content
    - id: readme
      static:
        filepath: README.md
        parse_as: raw

    # Inline JSON value
    - id: metadata
      static:
        value: |
          {"version": "1.0.0", "environment": "production"}
        parse_as: json

    # Inline text value
    - id: banner
      static:
        value: "Welcome to the system"
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

#### Archive to .tar.gz (Filesystem)

Bundle all step outputs into a single compressed archive:

```yaml
kind: CollectJob
metadata:
  name: archive-test
spec:
  collectors:
    - id: api
      http:
        base_url: https://api.example.com
  steps:
    - id: users
      collector: api
      http_get:
        path: /users
    - id: posts
      collector: api
      http_get:
        path: /posts
  output:
    encoding:
      json:
        indent: "  "
    archive:
      format: tar
      compression: gzip
      name: $JOB_NAME-$JOB_DATE_ISO8601
    sink:
      filesystem:
        path: ./output
        prefix: $JOB_NAME/$JOB_DATE_RFC3339
```

This produces `./output/archive-test/2026-01-24T12:00:00Z/archive-test-20260124T120000Z.tar.gz` containing `users.json`
and `posts.json`. Use `compression: zstd` for `.tar.zst` or `compression: none` for `.tar`.

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

#### S3 Sink - AWS S3

Using AWS S3 with environment/IAM credentials:

```yaml
kind: CollectJob
metadata:
  name: aws-inventory
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
  output:
    sink:
      s3:
        bucket: my-infracollect-bucket
        region: us-west-2
        prefix: $JOB_NAME/$JOB_DATE_ISO8601
```

#### S3 Sink - Cloudflare R2

```yaml
kind: CollectJob
metadata:
  name: r2-export
spec:
  collectors:
    - id: api
      http:
        base_url: https://api.example.com
  steps:
    - id: data
      collector: api
      http_get:
        path: /data
  output:
    sink:
      s3:
        bucket: my-r2-bucket
        endpoint: https://ACCOUNT_ID.r2.cloudflarestorage.com
        prefix: exports/$JOB_NAME
        credentials:
          access_key_id: ${R2_ACCESS_KEY_ID}
          secret_access_key: ${R2_SECRET_ACCESS_KEY}
```

#### S3 Sink - MinIO

```yaml
kind: CollectJob
metadata:
  name: local-export
spec:
  collectors:
    - id: k8s
      terraform:
        provider: hashicorp/kubernetes
        args:
          config_path: ~/.kube/config
  steps:
    - id: deployments
      collector: k8s
      terraform_datasource:
        name: kubernetes_resources
        args:
          api_version: apps/v1
          kind: Deployment
  output:
    sink:
      s3:
        bucket: local-bucket
        endpoint: http://localhost:9000
        force_path_style: true
        credentials:
          access_key_id: minioadmin
          secret_access_key: minioadmin
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

Provider arguments can reference environment variables using `${VARIABLE_NAME}` syntax. The system will substitute these
values at runtime.

## Best Practices

1. **Naming**: Use descriptive names for collectors and steps
2. **Organization**: Group related steps together
3. **Secrets**: Use environment variables for sensitive data
4. **Versioning**: Pin provider versions for reproducibility
5. **Documentation**: Add descriptions to complex pipelines
6. **Validation**: Validate pipelines before execution
7. **Output Format**: Use pretty-printed JSON (`indent: "  "`) for human-readable output, compact JSON for machine
   processing
8. **Output Destination**: Use `filesystem` for local development and debugging, `stdout` for piping to other tools or
   streaming results, `s3` for cloud storage
9. **File Organization**: Use the `prefix` field with `$JOB_NAME` and `$JOB_DATE_ISO8601` variables to organize outputs
   by job and timestamp. Prefer `$JOB_DATE_ISO8601` over `$JOB_DATE_RFC3339` as it avoids colons which require URL
   encoding
10. **Result Structure**: Each result includes an `id` field identifying the step that produced it, along with the
    `data` field containing the actual result data
11. **S3 Credentials**: For AWS S3, prefer using IAM roles or environment variables over explicit credentials in the job
    file. Use explicit credentials only for non-AWS S3-compatible services (R2, MinIO)
12. **Archive**: Use `output.archive` to produce a single `.tar.gz` or `.tar.zst` file when you need to ship or store a
    complete snapshot. Archive requires a filesystem or S3 sink. Use `name: $JOB_NAME-$JOB_DATE_ISO8601` for unique,
    sortable filenames
