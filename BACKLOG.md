# Backlog

This file tracks ideas, tasks, and work items for infracollect. Items are organized by priority.

**Status Legend:**

- `[ ]` Pending
- `[~]` In Progress
- `[x]` Done

---

## P0 - Critical (v0 Release)

<!-- Items that block other work or are urgent fixes -->

### [ ] Basic test coverage

Minimum test coverage for core components:

- `internal/runner/run_test.go` - Job parsing and validation
- `internal/engine/pipeline_test.go` - Pipeline execution
- `internal/collectors/terraform/collector_test.go` - Terraform collector

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

### [ ] Environment variables

- Support for environment variables in the job file

### [ ] Compressor

- Support .tar.gz/tar.zst compression

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
- Show line numbers for YAML parse errors
- Clear messages for missing required fields

### [ ] Subprocess collector

The subprocess collector is a collector that runs a subprocess and captures the output, this will enable unlimited possibilities for collectors.

---

## P2 - Medium Priority

<!-- Nice-to-have features and enhancements -->

### [ ] Adopt builder pattern for pipeline

The pipeline is currently built using a series of functions that build the different components of the pipeline. This is not ideal because it makes it difficult to understand the pipeline and to modify it.

### [ ] Advanced DAG engine

Steps are sequential and cannot be executed in parallel.
Nested steps would be useful to transform the data from one step to the next.

---

## P3 - Low Priority / Ideas

<!-- Future ideas and nice-to-haves -->

### [ ] More template pattern

Evalute goexpr or gotemplate

### [ ] Integration tests with testcontainers

Test with Kind, RustFS, etc... for the different collectors.

---

## Done

<!-- Move completed items here with completion date -->
