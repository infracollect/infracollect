# Infracollect Roadmap

This document tracks work needed to get the project into production-ready shape.

## CI/CD

### GitHub Actions CI Pipeline

- [x] **Lint**: Run `golangci-lint` on PRs and pushes
- [x] **Test**: Run `go test ./...` with race detection
- [x] **Build**: Verify the project builds on Linux and macOS
- [ ] **Code coverage**: Generate and upload coverage reports (Codecov or Coveralls)
- [x] **Security scanning**: Run `govulncheck` for vulnerability detection
- [ ] **Dependency review**: Check for license compliance and vulnerabilities on PRs

### Release Automation

- [ ] **GoReleaser setup**: Configure `.goreleaser.yaml` for automated releases
- [ ] **Binary releases**: Build binaries for Linux (amd64, arm64) and macOS (amd64, arm64)
- [ ] **Checksums**: Generate SHA256 checksums for all artifacts
- [ ] **Changelog generation**: Auto-generate changelog from conventional commits
- [ ] **GitHub Release**: Publish releases with release notes

### Container Images

- [x] **Dockerfile**: Create multi-stage Dockerfile for minimal image
- [ ] **GitHub Container Registry**: Push images to `ghcr.io`
- [x] **Multi-arch images**: Build for linux/amd64 and linux/arm64
- [ ] **Image signing**: Sign images with Cosign/Sigstore

---

## Documentation

### Project Website (GitHub Pages)

- [ ] **Static site generator**: Set up with Hugo, MkDocs, or Docusaurus
- [ ] **Auto-deploy**: Deploy to gh-pages on release or main push
- [ ] **Custom domain** (optional): Configure if desired

### Documentation Content

- [ ] **Getting started guide**: Installation, basic usage, first job
- [ ] **Configuration reference**: Document all job spec options
- [ ] **Collector documentation**: Document each collector type and its options
- [ ] **Step documentation**: Document available steps and their configuration
- [ ] **Examples**: Collection of example job files for common use cases
- [ ] **Architecture overview**: How the pipeline system works

### API Documentation

- [ ] **Go package docs**: Ensure all exported types/functions are documented
- [ ] **pkg.go.dev**: Verify package is indexed on pkg.go.dev
- [ ] **JSON Schema**: Generate and publish JSON Schema for job files
- [ ] **OpenAPI spec** (if API is added): Auto-generate from code

---

## Code Quality

### Linting & Formatting

- [ ] **golangci-lint config**: Create `.golangci.yml` with appropriate linters
- [ ] **gofumpt**: Enforce stricter formatting
- [ ] **Pre-commit hooks**: Set up with pre-commit framework

### Testing

- [ ] **Unit test coverage**: Target >80% coverage for core packages
- [ ] **Integration tests**: Test full pipeline execution
- [ ] **Test fixtures**: Organize test data in `testdata/` directories
- [ ] **Fuzz testing**: Add fuzz tests for parsers

### Code Organization

- [x] **Mise**: Add targets for common tasks (build, test, lint, generate)
- [ ] **go generate**: Document and automate code generation steps

---

## Project Hygiene

### Repository Setup

- [x] **LICENSE**: Add appropriate license file (MIT, Apache 2.0, etc.)
- [ ] **CONTRIBUTING.md**: Guidelines for contributors
- [ ] **SECURITY.md**: Security policy and vulnerability reporting
- [ ] **CODE_OF_CONDUCT.md**: Community guidelines
- [ ] **Issue templates**: Bug report, feature request templates
- [ ] **PR template**: Pull request template with checklist

### Dependency Management

- [ ] **Dependabot or Renovate**: Automated dependency updates
- [ ] **go.mod tidy**: Ensure clean dependency tree
- [ ] **Vendor** (optional): Decide on vendoring strategy

### README Improvements

- [ ] **Badges**: Build status, coverage, Go version, license
- [ ] **Installation instructions**: go install, binary download, Docker
- [ ] **Quick start example**: Minimal working example
- [ ] **Feature list**: Overview of capabilities

---

## Distribution

### Package Managers

- [ ] **Homebrew tap**: Create formula for macOS/Linux installation
- [ ] **AUR package** (optional): Arch Linux package
- [ ] **Scoop manifest** (optional): Windows package manager
- [ ] **Nix flake** (optional): Nix package

### Container Registry

- [ ] **Docker Hub** (optional): Mirror to Docker Hub
- [ ] **Versioned tags**: Tag images with semver (v1.0.0, v1.0, v1, latest)

---

## Future Enhancements

### Observability

- [ ] **Structured logging**: Ensure consistent log format
- [ ] **Metrics**: Prometheus metrics for pipeline execution
- [ ] **Tracing**: OpenTelemetry support for distributed tracing

### Features

- [ ] **Plugin system**: Allow external collectors/steps
- [ ] **Config validation CLI**: `infracollect validate` command
- [ ] **Dry-run mode**: Preview what a job would do without executing
- [ ] **Watch mode**: Re-run collection on schedule or file changes

---

## Priority Order

**Phase 1 - Foundation**

1. LICENSE file
2. Mise with basic targets (build, test, lint, generate)
3. golangci-lint configuration
4. Basic GitHub Actions CI (lint, test, build)

**Phase 2 - Release Infrastructure**

1. GoReleaser configuration
2. Dockerfile
3. Release workflow (on tag push)
4. README badges and installation instructions

**Phase 3 - Documentation**

1. Documentation site setup
2. Getting started guide
3. Configuration reference
4. Auto-deploy documentation

**Phase 4 - Polish**

1. Homebrew tap
2. Pre-commit hooks
3. Issue/PR templates
4. CONTRIBUTING.md and other community files
