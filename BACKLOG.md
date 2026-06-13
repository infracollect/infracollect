# Backlog

This file tracks ideas, tasks, and work items for infracollect. Items are organized by priority.

**Status Legend:**

- `[ ]` Pending
- `[~]` In Progress
- `[x]` Done

---

## P0 - Critical (v0 Release)

<!-- Items that block other work or are urgent fixes -->

### [x] Basic test coverage

Minimum test coverage for core components:

- `internal/runner/run_test.go` - Job parsing and validation
- `internal/engine/pipeline_test.go` - Pipeline execution
- `internal/integrations/terraform/collector_test.go` - Terraform collector

### [x] LICENSE file

Add a LICENSE file (MIT or Apache-2.0).

### [ ] Complete README

Update README.md with:

- Installation instructions
- Quick start guide
- Basic usage examples

### [x] CI pipeline

GitHub Actions workflow with:

- `go test ./...`
- `go build ./cmd/infracollect`
- Run on PRs and main branch

---

## P1 - High Priority

<!-- Core features and important improvements -->

### [x] Validate command

Add a `validate` command to validate the job file.

### [x] Environment variables

- Support for environment variables in the job file

### [x] Archive support

- Support .tar.gz/tar.zst compression via `output.archive` configuration

### [ ] GoReleaser configuration

Set up GoReleaser for binary distribution:

- Cross-platform builds (linux, darwin, windows)
- Checksums
- GitHub releases

### [ ] Version flag

Add `--version` flag to CLI and read from Go runtime

### [ ] User-friendly error messages

Improve validation error messages to be more actionable:

- "collector 'aws' not found" → suggest similar names
- Show source ranges for HCL parse errors
- Clear messages for missing required fields

### [ ] Exec step improvements

The exec step already covers the subprocess use case. Improve it with:

- Reusable exec "profiles" to avoid repeating program/env config across steps
- Streaming support for long-running commands
- Better error formatting (structured stderr parsing)

---

## P2 - Medium Priority

<!-- Nice-to-have features and enhancements -->

### [ ] Contributor documentation: Adding a new collector

Create documentation for contributors on how to add a new collector/step:

- Defining HCL config structs with `hcl:"..."` tags
- Implementing `engine.Collector` or `engine.Step` interfaces
- Registering factories in the registry
- Example walkthrough of adding a complete collector with steps

### [ ] Adopt builder pattern for pipeline

The pipeline is currently built using a series of functions that build the different components of the pipeline. This is
not ideal because it makes it difficult to understand the pipeline and to modify it.

### [~] Advanced DAG engine

Steps are sequential and cannot be executed in parallel.
Nested steps would be useful to transform the data from one step to the next.

---

## P3 - Low Priority / Ideas

<!-- Future ideas and nice-to-haves -->

### [ ] SSRF protection for remote job files

Add protection against Server-Side Request Forgery when fetching remote job files. Block requests to private IP ranges
(10.x, 172.16-31.x, 192.168.x, 127.x, link-local, etc.) to prevent access to internal services.

### [ ] Structured value for static steps

Support HCL object literals in static steps to allow passing structured data directly, avoiding string escaping:

```hcl
step "static" "config" {
  value = {
    foo = "bar"
    nested = {
      key = "value"
    }
  }
}
```

### [ ] YAML parsing for static steps

Add `parse_as = "yaml"` option for static steps to support YAML files. Auto-detect by `.yaml`/ `.yml` extension like
JSON.

### [ ] Glob patterns for static steps

Allow `filepath = "data/*.json"` to load multiple files in a single static step. Each matched file becomes a separate
entry in the result.

### [ ] Integration tests with testcontainers

Test with Kind, RustFS, etc... for the different collectors.

---

## Done

<!-- Move completed items here with completion date -->

- [x] **Archive support** - Added .tar.gz/tar.zst compression via `output.archive` configuration (completed 2026-01-24)
- [x] **Environment variables** - Support for environment variables in job files via `--allowed-env` flag and template
  expansion (completed 2026-01-26)
- [x] **Validate command** - Added `validate` command with pretty error formatting for validation and YAML errors
  (completed 2026-01-26)
- [x] **Basic test coverage** - Added enginetest shared helpers, encoder tests, hclfuncs tests, HTTP collector tests,
  and end-to-end runner tests (completed 2026-04-13)
- [x] **Dry-run mode** - Added a `plan` command that prints the resolved execution plan (topological node order,
  dependencies, collector bindings, output config) without running collectors or steps. `for_each` cardinality is
  resolved at plan time when statically knowable, otherwise marked computed-at-runtime. Implemented as a separate
  command rather than a `collect --dry-run` flag to fit the automation-first (k8s/cronjob) usage (completed
  2026-06-13)
- [x] **Extract the runner `scope` seam** - Pulled the `step.*`/`collector.*` cty namespaces and all per-node
  `hcl.EvalContext` construction out of `Runner` into a dedicated `scope` module (`internal/runner/scope.go`).
  Collapsed the duplicated step-run logic in `runStep`/`runCollection` into a shared `executeStepOnce` kernel.
  Context-building is now unit-testable in isolation (`scope_test.go`) instead of only through a full `Run`
  (completed 2026-06-13)
- [x] **Deepen the output path into a `ResultWriter`** - Folded the encode + file-naming (`<id>.<ext>`,
  `<id>.meta.<ext>`) + meta rule + close ordering out of `Runner.writeResults` into a concrete
  `engine.ResultWriter` (`internal/engine/resultwriter.go`) over the `Encoder`/`Sink` seams. The Runner now only
  decides which results and in what order; the write path is unit-testable with a fake encoder + sink
  (`resultwriter_test.go`). `buildResultWriter` wraps the existing `buildOutputPipeline` HCL decode (completed
  2026-06-13)
