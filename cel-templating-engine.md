# CEL Templating Engine Design Document

## Overview

Kro (Kubernetes Resource Orchestrator) uses a CEL (Common Expression Language) templating engine to enable dynamic resource composition. This document explains how YAML resource templates are parsed, how CEL expressions are extracted and evaluated, and how values are resolved into final Kubernetes resources.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Expression Syntax](#expression-syntax)
3. [Step 1: YAML Parsing and Expression Extraction](#step-1-yaml-parsing-and-expression-extraction)
4. [Step 2: Schema Resolution and Type Conversion](#step-2-schema-resolution-and-type-conversion)
5. [Step 3: CEL Environment Setup](#step-3-cel-environment-setup)
6. [Step 4: Dependency Graph Construction](#step-4-dependency-graph-construction)
7. [Step 5: CEL Expression Validation](#step-5-cel-expression-validation)
8. [Step 6: Runtime Resolution](#step-6-runtime-resolution)
9. [Field Path System](#field-path-system)
10. [Variable Classification](#variable-classification)
11. [Special Expression Types](#special-expression-types)
12. [Type System](#type-system)
13. [Error Handling](#error-handling)

---

## Architecture Overview

The CEL templating engine consists of several interconnected layers that transform a ResourceGraphDefinition into deployable Kubernetes resources:

```
┌─────────────────────────────────────────────────────────────────┐
│  ResourceGraphDefinition YAML (Input)                           │
│  - Contains ${...} CEL expressions in resource templates        │
│  - Defines schema for user parameters                           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 1: Parser Layer (pkg/graph/parser/)                       │
│  - Extracts ${...} expressions from YAML fields                 │
│  - Validates resource structure against OpenAPI schemas         │
│  - Produces FieldDescriptor objects with path info              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 2: Schema Resolution (pkg/graph/schema/resolver/)         │
│  - Resolves OpenAPI schemas for each resource GVK               │
│  - Combines core K8s schemas with CRD schemas                   │
│  - Provides type information for validation                     │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 3: CEL Environment Setup (pkg/cel/)                       │
│  - Converts OpenAPI schemas to CEL DeclTypes                    │
│  - Creates typed CEL environment with variable declarations     │
│  - Enables compile-time type checking                           │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 4: Dependency Graph Construction (pkg/graph/builder.go)   │
│  - Inspects CEL AST to find resource references                 │
│  - Builds DAG of resource dependencies                          │
│  - Computes topological ordering for creation                   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 5: CEL Expression Validation                              │
│  - Type-checks all expressions against schemas                  │
│  - Validates output types match expected field types            │
│  - Reports detailed type mismatch errors                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Step 6: Runtime Resolution (pkg/runtime/resolver/)             │
│  - Evaluates CEL expressions with actual data                   │
│  - Substitutes values into resource templates                   │
│  - Produces final Kubernetes resource manifests                 │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Kubernetes Resources (Output)                                  │
│  - Fully instantiated, ready for creation                       │
└─────────────────────────────────────────────────────────────────┘
```

---

## Expression Syntax

CEL expressions in kro are wrapped in `${...}` delimiters:

### Standalone Expressions

A standalone expression is a single CEL expression that replaces the entire field value:

```yaml
replicas: ${schema.spec.replicas}        # Evaluates to integer
enabled: ${schema.spec.feature.enabled}  # Evaluates to boolean
config: ${configMap.data}                # Evaluates to object/map
```

The output type is determined by the CEL expression's return type, which must match the expected schema type for that field.

### String Templates

String templates contain text mixed with one or more expressions:

```yaml
name: app-${schema.spec.name}                    # "app-myservice"
url: https://${host}:${port}/api                 # "https://api.example.com:8080/api"
label: ${schema.spec.env}-${schema.spec.region}  # "prod-us-west-2"
```

String templates always produce string output, regardless of the expression types within them.

### Detection Logic

The parser (`pkg/graph/parser/cel.go:108-117`) determines expression type:

```go
func isStandaloneExpression(str string) (bool, error) {
    expressions, err := extractExpressions(str)
    if err != nil {
        return false, err
    }
    // Standalone if exactly one expression AND it's the entire string
    return len(expressions) == 1 && str == exprStart+expressions[0]+exprEnd, nil
}
```

---

## Step 1: YAML Parsing and Expression Extraction

**Location:** `pkg/graph/parser/parser.go`

### Entry Point

The `ParseResource` function recursively traverses the YAML structure:

```go
func ParseResource(resource map[string]interface{}, resourceSchema *spec.Schema) ([]variable.FieldDescriptor, error) {
    return parseResource(resource, resourceSchema, "")
}
```

### Recursive Traversal

The parser handles different YAML node types:

```go
func parseResource(resource interface{}, schema *spec.Schema, path string) ([]variable.FieldDescriptor, error) {
    switch field := resource.(type) {
    case map[string]interface{}:
        return parseObject(field, schema, path, expectedTypes)
    case []interface{}:
        return parseArray(field, schema, path, expectedTypes)
    case string:
        return parseString(field, path, expectedTypes)
    // ... handle other types
    }
}
```

### Expression Extraction

For string fields, expressions are extracted (`pkg/graph/parser/cel.go:31-106`):

```go
func extractExpressions(str string) ([]string, error) {
    var expressions []string

    for start < len(str) {
        // Find "${" start marker
        startIdx := strings.Index(str[start:], exprStart)
        if startIdx == -1 {
            break
        }

        // Find matching "}" considering:
        // - Nested braces in CEL (e.g., map literals)
        // - String literals (don't count braces inside quotes)
        // - Escape sequences

        bracketCount := 1
        inStringLiteral := false

        for endIdx < len(str) {
            c := str[endIdx]

            if c == '"' {
                inStringLiteral = !inStringLiteral
            } else if !inStringLiteral {
                if c == '{' {
                    bracketCount++
                } else if c == '}' {
                    bracketCount--
                    if bracketCount == 0 {
                        break  // Found matching close
                    }
                }
            }
            endIdx++
        }

        expr := str[startIdx+len(exprStart) : endIdx]
        expressions = append(expressions, expr)
    }
    return expressions, nil
}
```

### Output: FieldDescriptor

For each field containing expressions, a `FieldDescriptor` is created:

```go
type FieldDescriptor struct {
    // JSONPath-like location: "spec.containers[0].env[0].value"
    Path string

    // Extracted CEL expressions (without ${} wrappers)
    Expressions []string

    // Expected CEL type (set later by builder)
    ExpectedType *cel.Type

    // true if single expression, false if string template
    StandaloneExpression bool
}
```

### Schema Validation During Parsing

The parser validates each field against the OpenAPI schema:

1. **Type Checking**: Verifies the YAML value type matches schema expectations
2. **Schema Extensions**: Handles `x-kubernetes-preserve-unknown-fields` and `x-kubernetes-int-or-string`
3. **Composite Schemas**: Processes `oneOf`, `anyOf` constructs

---

## Step 2: Schema Resolution and Type Conversion

**Location:** `pkg/graph/schema/resolver/`

### Combined Resolver

The `CombinedResolver` merges multiple schema sources:

```go
func NewCombinedResolver(clientConfig *rest.Config, httpClient *http.Client) (resolver.SchemaResolver, error) {
    // 1. Core resolver: compiled-in Kubernetes OpenAPI definitions
    // 2. Client resolver: discovers CRD schemas via API server
    // 3. TTL cache: 500 entries, 5-minute expiry
}
```

### Schema Lookup

For each resource, the schema is resolved by GVK:

```go
resourceSchema, err := b.schemaResolver.ResolveSchema(gvk)
```

### OpenAPI to CEL Type Conversion

**Location:** `pkg/cel/schemas.go`

The `SchemaDeclTypeWithMetadata` function converts OpenAPI schemas to CEL types:

```go
func SchemaDeclTypeWithMetadata(s common.Schema, isResourceRoot bool) *apiservercel.DeclType {
    switch s.Type() {
    case "string":
        // Handle formats: byte, duration, date, date-time
        return apiservercel.NewSimpleTypeWithMinSize("string", cel.StringType, ...)

    case "integer":
        return apiservercel.IntType

    case "number":
        return apiservercel.DoubleType

    case "boolean":
        return apiservercel.BoolType

    case "array":
        itemsType := SchemaDeclTypeWithMetadata(s.Items(), ...)
        return apiservercel.NewListType(itemsType, maxItems)

    case "object":
        if s.AdditionalProperties() != nil {
            // Map type: map[string]valueType
            return apiservercel.NewMapType(apiservercel.StringType, propsType, maxProperties)
        }
        // Struct type with defined fields
        fields := make(map[string]*apiservercel.DeclField)
        for name, prop := range s.Properties() {
            fieldType := SchemaDeclTypeWithMetadata(prop, ...)
            fields[name] = apiservercel.NewDeclField(name, fieldType, required, ...)
        }
        return apiservercel.NewObjectType("object", fields)
    }
}
```

### Type Mapping Table

| OpenAPI Type | CEL Type | Example Value |
|--------------|----------|---------------|
| `string` | `cel.StringType` | `"hello"` |
| `integer` | `cel.IntType` | `42` |
| `number` | `cel.DoubleType` | `3.14` |
| `boolean` | `cel.BoolType` | `true` |
| `array` | `cel.ListType(elemType)` | `[1, 2, 3]` |
| `object` (properties) | `cel.StructType` | `{name: "x", value: 1}` |
| `object` (additionalProperties) | `cel.MapType(string, T)` | `{"key": "value"}` |
| `x-kubernetes-int-or-string` | `cel.DynType` | `"50%"` or `50` |
| `x-kubernetes-preserve-unknown-fields` | `cel.DynType` | any value |

---

## Step 3: CEL Environment Setup

**Location:** `pkg/cel/environment.go`

### Default Environment

The CEL environment is configured with libraries and variable declarations:

```go
func DefaultEnvironment(options ...EnvOption) (*cel.Env, error) {
    declarations := []cel.EnvOption{
        ext.Lists(),           // List manipulation functions
        ext.Strings(),         // String manipulation functions
        cel.OptionalTypes(),   // Optional type support
        ext.Encoders(),        // JSON/Base64 encoding
        k8scellib.URLs(),      // URL parsing (Kubernetes library)
        k8scellib.Regex(),     // Regex matching (Kubernetes library)
        library.Random(),      // Random number generation (kro library)
    }

    // Add typed resource declarations
    if len(opts.typedResources) > 0 {
        for name, schema := range opts.typedResources {
            declType := SchemaDeclTypeWithMetadata(&openapi.Schema{Schema: schema}, false)
            typeName := TypeNamePrefix + name  // e.g., "__type_schema"
            declType = declType.MaybeAssignTypeName(typeName)

            // Register type provider for field resolution
            declTypes = append(declTypes, declType)

            // Declare variable with the schema type
            declarations = append(declarations, cel.Variable(name, declType.CelType()))
        }

        // Create custom type provider for struct field access
        baseProvider := NewDeclTypeProvider(declTypes...)
        baseProvider.SetRecognizeKeywordAsFieldName(true)  // Allow "namespace" etc.
        declarations = append(declarations, cel.CustomTypeProvider(wrappedProvider))
    }

    return cel.NewEnv(declarations...)
}
```

### Type Provider

The `DeclTypeProvider` enables CEL to resolve field access on custom types:

```go
type DeclTypeProvider struct {
    typeMap                      map[string]*apiservercel.DeclType
    recognizeKeywordAsFieldName  bool
}
```

This allows expressions like `schema.spec.replicas` to be type-checked against the actual schema structure.

---

## Step 4: Dependency Graph Construction

**Location:** `pkg/graph/builder.go:421-483`

### AST Inspection

Dependencies are extracted by inspecting the CEL AST:

```go
func (b *Builder) buildDependencyGraph(nodes map[string]*Node) (*dag.DirectedAcyclicGraph[string], error) {
    // Create CEL environment with all node names
    nodeNames := append(maps.Keys(nodes), SchemaVarName)
    env, _ := krocel.DefaultEnvironment(krocel.WithResourceIDs(nodeNames))

    // Build DAG
    dag := dag.NewDirectedAcyclicGraph[string]()
    for _, node := range nodes {
        dag.AddVertex(node.Meta.ID, node.Meta.Index)
    }

    // Extract dependencies for each node
    for _, node := range nodes {
        for _, templateVariable := range node.Variables {
            for _, expression := range templateVariable.Expressions {
                deps, _, _ := extractDependencies(env, expression, nodeNames, iteratorNames)
                // deps contains referenced resource IDs
            }
        }
        dag.AddDependencies(node.Meta.ID, allDeps)
    }
}
```

### AST Inspector

**Location:** `pkg/cel/ast/inspector.go`

The inspector walks the CEL AST to find variable references:

```go
type ExpressionInspection struct {
    ResourceDependencies []ResourceDependency  // Found resource references
    FunctionCalls        []FunctionCall         // Called functions
    UnknownResources     []UnknownResource     // Undefined references
    UnknownFunctions     []UnknownFunction     // Undefined functions
}
```

### Topological Sort

The DAG is sorted to determine creation order:

```go
topologicalOrder, err := dag.TopologicalSort()
// Returns: ["resourcegroup", "storageAccount", "container", ...]
```

Resources are created in this order, ensuring dependencies are available.

---

## Step 5: CEL Expression Validation

**Location:** `pkg/graph/builder.go:980-1078`

### Template Expression Validation

Each expression is parsed, type-checked, and validated:

```go
func validateTemplateExpressions(env *cel.Env, node *Node, typeProvider *krocel.DeclTypeProvider) error {
    for _, templateVariable := range node.Variables {
        expression := templateVariable.Expressions[0]

        // Parse and type-check
        checkedAST, err := parseAndCheckCELExpression(env, expression)
        if err != nil {
            return fmt.Errorf("failed to type-check: %w", err)
        }

        // Validate output type matches expected
        outputType := checkedAST.OutputType()
        err = validateExpressionType(outputType, templateVariable.ExpectedType, ...)
    }
}
```

### Type Compatibility Checking

**Location:** `pkg/cel/compatibility.go`

The type checker performs structural compatibility analysis:

```go
func AreTypesStructurallyCompatible(outputType, expectedType *cel.Type, provider *DeclTypeProvider) (bool, error) {
    // 1. Check CEL's built-in nominal type checking
    if expectedType.IsAssignableType(outputType) {
        return true, nil
    }

    // 2. Handle map ↔ struct compatibility (duck typing)
    // 3. Handle optional type unwrapping
    // 4. Recursively check nested types
}
```

### Condition Expression Validation

`includeWhen` and `readyWhen` expressions must return boolean:

```go
func validateConditionExpression(env *cel.Env, expression, conditionType, resourceID string) error {
    checkedAST, _ := parseAndCheckCELExpression(env, expression)

    outputType := checkedAST.OutputType()
    if !krocel.IsBoolOrOptionalBool(outputType) {
        return fmt.Errorf("%s must return bool, got %q", conditionType, outputType)
    }
}
```

---

## Step 6: Runtime Resolution

**Location:** `pkg/runtime/resolver/resolver.go`

### Resolver Structure

```go
type Resolver struct {
    resource map[string]interface{}  // Original resource template
    data     map[string]interface{}  // Evaluated expression values
}
```

### Resolution Process

```go
func (r *Resolver) Resolve(expressions []variable.FieldDescriptor) ResolutionSummary {
    for _, field := range expressions {
        result := r.resolveField(field)
    }
}

func (r *Resolver) resolveField(field variable.FieldDescriptor) ResolutionResult {
    if field.StandaloneExpression {
        // Replace entire field with evaluated value
        expr := field.Expressions[0]
        resolvedValue := r.data[expr]  // Pre-evaluated by CEL
        r.setValueAtPath(field.Path, resolvedValue)
    } else {
        // String template: replace each ${expr} inline
        strValue := r.getValueFromPath(field.Path)
        for _, expr := range field.Expressions {
            replacement := r.data[expr]
            strValue = strings.ReplaceAll(strValue, "${"+expr+"}", fmt.Sprintf("%v", replacement))
        }
        r.setValueAtPath(field.Path, strValue)
    }
}
```

### Path Navigation

Setting values at nested paths handles array creation/resizing:

```go
func (r *Resolver) setValueAtPath(path string, value interface{}) error {
    segments, _ := fieldpath.Parse(path)

    for i, segment := range segments {
        if segment.Index >= 0 {
            // Array access - create/resize array if needed
            array := current.([]interface{})
            if segment.Index >= len(array) {
                newArray := make([]interface{}, segment.Index+1)
                copy(newArray, array)
                // Update parent reference
            }
        } else {
            // Object access - create map if needed
            if currentMap[segment.Name] == nil {
                currentMap[segment.Name] = make(map[string]interface{})
            }
        }
    }
}
```

---

## Field Path System

**Location:** `pkg/graph/fieldpath/parser.go`

### Path Format

Field paths use a JSONPath-like syntax:

```
spec.containers[0].env[0].value
metadata.labels["app.kubernetes.io/name"]
spec["my.field.name"].items[0]
```

### Segment Types

```go
type Segment struct {
    Name  string  // Field name (empty for pure index access)
    Index int     // Array index (-1 if not array access)
}
```

### Parsing Rules

1. **Unquoted fields**: `spec`, `containers` - parsed until `.` or `[`
2. **Quoted fields**: `["my.field.name"]` - for fields with special characters
3. **Array indices**: `[0]`, `[1]` - numeric access

```go
func Parse(path string) ([]Segment, error) {
    // Parse: spec["my.field"][0].name
    // Result: [
    //   {Name: "spec", Index: -1},
    //   {Name: "my.field", Index: -1},
    //   {Name: "", Index: 0},
    //   {Name: "name", Index: -1}
    // ]
}
```

---

## Variable Classification

**Location:** `pkg/graph/variable/variable.go`

### Variable Kinds

```go
const (
    // Static: references only schema.spec.* (user parameters)
    // Resolved once at the start, value never changes
    ResourceVariableKindStatic ResourceVariableKind = "static"

    // Dynamic: references other resources
    // Must wait for dependencies to be created
    ResourceVariableKindDynamic ResourceVariableKind = "dynamic"

    // ReadyWhen: readiness condition for the resource itself
    // Polled until returning true
    ResourceVariableKindReadyWhen ResourceVariableKind = "readyWhen"

    // IncludeWhen: conditional resource creation
    // Evaluated once to decide if resource should exist
    ResourceVariableKindIncludeWhen ResourceVariableKind = "includeWhen"

    // Iteration: references forEach iterator variables
    // Evaluated once per collection item
    ResourceVariableKindIteration ResourceVariableKind = "iteration"
)
```

### Classification Logic

Variables are classified during dependency extraction:

```go
// Start as Static
templateVariable.Kind = variable.ResourceVariableKindStatic

// Promote to Iteration if references iterator
if len(iteratorRefs) > 0 {
    templateVariable.Kind = variable.ResourceVariableKindIteration
}

// Promote to Dynamic if references other resources
if len(nodeDeps) > 0 && templateVariable.Kind == variable.ResourceVariableKindStatic {
    templateVariable.Kind = variable.ResourceVariableKindDynamic
}
```

---

## Special Expression Types

### includeWhen

Conditional resource creation based on user parameters:

```yaml
resources:
- id: s3PolicyWrite
  includeWhen:
  - ${schema.spec.access == "write"}
  template:
    # Only created if access == "write"
```

**Constraints:**
- Can only reference `schema.spec.*` (not other resources)
- Must return `bool` or `optional_type(bool)`
- Evaluated once during resource planning

### readyWhen

Wait conditions before considering a resource ready:

```yaml
resources:
- id: database
  template: ...
  readyWhen:
  - ${database.status.phase == "Running"}
  - ${database.status.connectionString != ""}
```

**Constraints:**
- Can only reference the resource itself (or `each` for collections)
- Must return `bool` or `optional_type(bool)`
- All conditions must be true for resource to be "ready"

### forEach

Create multiple resources from a list:

```yaml
resources:
- id: deployments
  forEach:
  - region: ${schema.spec.regions}
  template:
    metadata:
      name: app-${region}
```

**Behavior:**
- `forEach` expression must return a list
- Template is instantiated once per list element
- Iterator variable (`region`) is bound in CEL environment

---

## Type System

### CEL Type Hierarchy

```
                cel.Type
                    │
    ┌───────────────┼───────────────┐
    │               │               │
Primitives      Composites      Special
    │               │               │
 IntType        ListType        DynType
 DoubleType     MapType         AnyType
 StringType     StructType      OptionalType
 BoolType
 BytesType
 DurationType
 TimestampType
```

### Expected Type Resolution

**Location:** `pkg/graph/builder.go:863-920`

For standalone expressions, the expected type is derived from the schema:

```go
func setExpectedTypeOnDescriptor(descriptor *variable.FieldDescriptor, rootSchema *spec.Schema, resourceID string) {
    if !descriptor.StandaloneExpression {
        // String templates always produce strings
        descriptor.ExpectedType = cel.StringType
        return
    }

    // Walk the schema following the field path
    segments, _ := fieldpath.Parse(descriptor.Path)
    schema, typeName, _ := resolveSchemaAndTypeName(segments, rootSchema, resourceID)

    // Convert schema to CEL type
    descriptor.ExpectedType = getCelTypeFromSchema(schema, typeName)
}
```

### Type Coercion

The system handles several type coercion scenarios:

1. **Map to Struct**: A map can be assigned to a struct field if keys match
2. **Optional Unwrapping**: `optional_type(T)` can be assigned to `T`
3. **Dyn Compatibility**: `dyn` accepts any type

---

## Error Handling

### Parser Errors

```go
// Nested expression error
var ErrNestedExpression = errors.New("nested expressions are not allowed unless inside string literals")

// Schema validation error
fmt.Errorf("expected %s type for path %s, got %s", expectedTypes, path, actualType)
```

### Type Checking Errors

```go
// Type mismatch
fmt.Errorf(
    "type mismatch in resource %q at path %q: expression %q returns type %q but expected %q",
    resourceID, path, expression, outputType, expectedType,
)

// Unknown resource reference
fmt.Errorf("found unknown resources in CEL expression: [%v]", unknownResources)

// Unknown function call
fmt.Errorf("found unknown functions in CEL expression: [%v]", unknownFunctions)
```

### Runtime Errors

```go
// Missing evaluated data
fmt.Errorf("no data provided for expression: %s", expr)

// Path navigation error
fmt.Errorf("expected map at path segment: %v", segment)
fmt.Errorf("array index out of bounds: %d", index)
```

---

## Example: Complete Flow

### Input ResourceGraphDefinition

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp.kro.run
spec:
  schema:
    spec:
      name: string | required=true
      replicas: integer | default=1
  resources:
  - id: deployment
    template:
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: ${schema.spec.name}
      spec:
        replicas: ${schema.spec.replicas}
        template:
          spec:
            containers:
            - name: app
              image: nginx:latest
  - id: service
    template:
      apiVersion: v1
      kind: Service
      metadata:
        name: ${deployment.metadata.name}-svc
      spec:
        selector:
          app: ${schema.spec.name}
```

### Processing Steps

**Step 1: Parse Expressions**

```
deployment:
  - Path: "metadata.name"
    Expressions: ["schema.spec.name"]
    StandaloneExpression: true

  - Path: "spec.replicas"
    Expressions: ["schema.spec.replicas"]
    StandaloneExpression: true

service:
  - Path: "metadata.name"
    Expressions: ["deployment.metadata.name"]
    StandaloneExpression: false  (has "-svc" suffix)

  - Path: "spec.selector.app"
    Expressions: ["schema.spec.name"]
    StandaloneExpression: true
```

**Step 2: Build Dependencies**

```
deployment: depends on [schema]  (static)
service: depends on [deployment] (dynamic)

Topological order: [deployment, service]
```

**Step 3: Set Expected Types**

```
deployment.metadata.name -> cel.StringType (from Deployment schema)
deployment.spec.replicas -> cel.IntType (from Deployment schema)
service.metadata.name -> cel.StringType (string template)
service.spec.selector.app -> cel.StringType (from Service schema)
```

**Step 4: Validate Types**

```
schema.spec.name (string) -> deployment.metadata.name (string) ✓
schema.spec.replicas (int) -> deployment.spec.replicas (int) ✓
deployment.metadata.name (string) in template -> service.metadata.name (string) ✓
schema.spec.name (string) -> service.spec.selector.app (string) ✓
```

**Step 5: Runtime Resolution**

Given instance:
```yaml
spec:
  name: "myapp"
  replicas: 3
```

Resolved deployment:
```yaml
metadata:
  name: "myapp"
spec:
  replicas: 3
```

After deployment creation, resolved service:
```yaml
metadata:
  name: "myapp-svc"  # from "${deployment.metadata.name}-svc"
spec:
  selector:
    app: "myapp"
```

---

## Key Files Reference

| Component | File | Purpose |
|-----------|------|---------|
| Expression Parser | `pkg/graph/parser/parser.go` | Recursive YAML parsing |
| CEL Extractor | `pkg/graph/parser/cel.go` | `${...}` extraction logic |
| Variable Types | `pkg/graph/variable/variable.go` | FieldDescriptor, ResourceField |
| Field Path | `pkg/graph/fieldpath/parser.go` | Path segment parsing |
| CEL Environment | `pkg/cel/environment.go` | Environment configuration |
| Schema Conversion | `pkg/cel/schemas.go` | OpenAPI to CEL types |
| Type Compatibility | `pkg/cel/compatibility.go` | Structural type checking |
| Graph Builder | `pkg/graph/builder.go` | Orchestrates all steps |
| AST Inspector | `pkg/cel/ast/inspector.go` | Dependency extraction |
| Runtime Resolver | `pkg/runtime/resolver/resolver.go` | Value substitution |
| Schema Resolver | `pkg/graph/schema/resolver/resolver.go` | OpenAPI schema lookup |
