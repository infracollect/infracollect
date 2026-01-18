# CollectJob YAML Specification

## Overview

A `CollectJob` is a YAML document that defines a collection job for gathering infrastructure resources using Terraform providers through the `tf-data-client` library.

## Schema

```yaml
kind: CollectJob
metadata:
  name: string          # Required: Pipeline name
  description?: string  # Optional: Pipeline description
spec:
  collectors:           # Required: List of collector definitions
    - id: string        # Required: Unique collector identifier
      terraform:
        provider: string    # Required: Provider name (e.g., "hashicorp/kubernetes")
        version?: string    # Optional: Provider version (e.g., "v2.32.0")
        args: object        # Required: Provider configuration arguments
  steps:                # Required: List of collection steps
    - id: string        # Required: Unique step identifier
      terraform_datasource:
        name: string        # Required: Terraform data source name
        collector: string   # Required: Collector ID to use
        args: object        # Required: Data source arguments
  output?:              # Optional: Output configuration
    encoding?:           # Optional: Encoding format configuration
      json?:             # Optional: JSON encoding options
        indent?: string  # Optional: Indentation (empty = compact, "  " = 2 spaces, "\t" = tabs)
    destination?:        # Optional: Output destination configuration
      stdout?: {}        # Optional: Write to stdout
      folder?:           # Optional: Write to folder (one file per step)
        path: string     # Required: Directory path
      zip?:              # Optional: Write to ZIP archive
        path: string     # Required: ZIP file path
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
- **Default**: If not specified, results are written to stdout as compact JSON

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

#### `spec.output.destination` (optional)
- **Type**: `object`
- **Description**: Configures where output is written
- **Default**: If not specified, results are written to stdout
- **Note**: Only one destination type should be specified

##### `spec.output.destination.stdout` (optional)
- **Type**: `object`
- **Description**: Write output to standard output
- **Note**: Currently has no configuration options (empty object `{}`)

##### `spec.output.destination.folder` (optional)
- **Type**: `object`
- **Description**: Write output to a folder with one file per step
- **File Naming**: Each step's output is written to a file named `{step-id}.json` (or appropriate extension based on encoding)

##### `spec.output.destination.folder.path` (required)
- **Type**: `string`
- **Description**: Directory path where output files will be written
- **Note**: Directory will be created if it doesn't exist

##### `spec.output.destination.zip` (optional)
- **Type**: `object`
- **Description**: Write output to a ZIP archive with one file per step

##### `spec.output.destination.zip.path` (required)
- **Type**: `string`
- **Description**: Path to the ZIP file to create
- **Note**: ZIP file will be created or overwritten if it exists

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

### Step Specification

#### `steps[].id` (required)
- **Type**: `string`
- **Description**: Unique identifier for the step
- **Constraints**: Must be unique within the pipeline

#### `steps[].terraform_datasource` (required)
- **Type**: `object`
- **Description**: Terraform data source configuration

##### `steps[].terraform_datasource.name` (required)
- **Type**: `string`
- **Description**: Terraform data source name
- **Examples**:
  - `kubernetes_resources`
  - `aws_instances`
  - `azurerm_resources`

##### `steps[].terraform_datasource.collector` (required)
- **Type**: `string`
- **Description**: ID of the collector to use for this step
- **Constraints**: Must reference a collector ID defined in `spec.collectors`

##### `steps[].terraform_datasource.args` (required)
- **Type**: `object`
- **Description**: Data source-specific arguments
- **Note**: Arguments vary by data source. Refer to provider documentation.

## Validation Rules

1. **Kind Validation**: `kind` must be `"CollectJob"`
2. **Metadata Validation**: `metadata.name` is required and must be a valid identifier
3. **Collector Validation**:
   - At least one collector must be defined
   - Each collector must have a unique `id`
   - Each collector must have `terraform.provider` and `terraform.args`
4. **Step Validation**:
   - At least one step must be defined
   - Each step must have a unique `id`
   - Each step must reference a valid collector ID
   - Each step must have `terraform_datasource.name` and `terraform_datasource.args`
5. **Reference Validation**: All collector references in steps must exist
6. **Output Validation**:
   - If `output.encoding` is specified, exactly one encoding type should be set
   - If `output.destination` is specified, exactly one destination type should be set
   - If `output.destination.folder` is specified, `path` is required
   - If `output.destination.zip` is specified, `path` is required

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
      terraform_datasource:
        name: kubernetes_resources
        collector: kind
        args:
          api_version: apps/v1
          kind: Deployment
          namespace: kube-system
    - id: pods
      terraform_datasource:
        name: kubernetes_resources
        collector: kind
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
      terraform_datasource:
        name: aws_instances
        collector: aws-us-east-1
        args:
          filters:
            - name: instance-state-name
              values: [running]
    - id: aws-west-instances
      terraform_datasource:
        name: aws_instances
        collector: aws-us-west-2
        args:
          filters:
            - name: instance-state-name
              values: [running]
    - id: azure-vms
      terraform_datasource:
        name: azurerm_virtual_machines
        collector: azure
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
      terraform_datasource:
        name: aws_instances
        collector: aws-prod
        args:
          filters:
            - name: tag:Environment
              values: [production]
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
      terraform_datasource:
        name: kubernetes_resources
        collector: kind
        args:
          api_version: apps/v1
          kind: Deployment
  output:
    encoding:
      json:
        indent: "  "
    destination:
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
      terraform_datasource:
        name: aws_instances
        collector: aws-prod
        args: {}
  output:
    encoding:
      json:
        indent: ""
    destination:
      folder:
        path: ./output
```

#### Pretty-Printed JSON to ZIP Archive

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
      terraform_datasource:
        name: aws_instances
        collector: aws
        args: {}
    - id: azure-vms
      terraform_datasource:
        name: azurerm_virtual_machines
        collector: azure
        args: {}
  output:
    encoding:
      json:
        indent: "\t"
    destination:
      zip:
        path: ./inventory.zip
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
      terraform_datasource:
        name: aws_instances
        collector: aws
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
8. **Output Destination**: Use `folder` for local development and debugging, `zip` for distribution, `stdout` for piping to other tools
