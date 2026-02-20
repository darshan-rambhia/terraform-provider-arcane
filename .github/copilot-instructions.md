# Copilot Instructions for terraform-provider-arcane

## Project Overview

A Terraform/OpenTofu provider for managing [Arcane](https://github.com/darshan-rambhia/arcane) — a Docker container management platform. Provides declarative infrastructure-as-code for container environments, deployments, and GitOps configurations.

## Architecture

```
main.go                          # Provider entrypoint (providerserver)
internal/
└── provider/
    ├── provider.go              # Provider config, API key auth
    ├── environment_resource.go  # arcane_environment CRUD
    ├── project_deployment_resource.go  # arcane_project_deployment
    ├── container_registry_resource.go  # arcane_container_registry
    ├── git_repository_resource.go      # arcane_git_repository
    ├── gitops_sync_resource.go         # arcane_gitops_sync
    ├── environment_data_source.go      # arcane_environment data source
    ├── project_data_source.go          # arcane_project data source
    ├── project_status_data_source.go   # arcane_project_status data source
    ├── environment_health_data_source.go # arcane_environment_health
    └── container_data_source.go        # arcane_container data source
examples/                        # Example Terraform configurations
docs/                            # Generated documentation (tfplugindocs)
```

## Development Commands (Taskfile)

```bash
task dev              # Build + install locally for terraform testing
task test:unit        # Fast unit tests
task lint             # Run golangci-lint
task docs             # Generate docs from schema
task release:dry-run  # Test goreleaser config locally
```

## Key Concepts

### Provider Authentication
The provider authenticates to Arcane via API key. On environment creation, an API key is auto-generated and stored in state.

### Resources
- `arcane_environment` — Docker daemon connection (environment) in Arcane
- `arcane_project_deployment` — Deploy/undeploy a project, tracks content hash for change detection
- `arcane_container_registry` — Container registry configuration
- `arcane_git_repository` — Git repository for GitOps workflows
- `arcane_gitops_sync` — GitOps sync configuration

### Data Sources
- `arcane_environment` — Look up environment by ID or name
- `arcane_project` — Look up project within an environment
- `arcane_project_status` — Get container-level details (health, ports, IDs)
- `arcane_environment_health` — Check agent connectivity
- `arcane_container` — Look up individual containers

## Code Conventions

### Terraform Plugin Framework
- Uses `terraform-plugin-framework` (NOT SDK v2)
- Resources implement `resource.Resource` and `resource.ResourceWithImportState`
- Schema uses `tfsdk` struct tags for model binding

### Error Handling
Use diagnostics for user-facing errors:
```go
resp.Diagnostics.AddError(
    "Summary of the error",
    "Detailed description with actionable guidance",
)
```

## CI/CD & Releasing

**GitHub Actions Workflows:**
- `.github/workflows/test.yml` — Runs on PR/push: lint, unit tests, acceptance tests (matrix TF 1.6-1.9 + OpenTofu 1.6-1.8)
- `.github/workflows/lint.yml` — Dedicated lint badge
- `.github/workflows/docs.yml` — Verify docs are up to date
- `.github/workflows/release.yml` — Manual dispatch: run tests → create tag → GoReleaser

**Release Process:**
1. Go to **Actions** > **Release** > **Run workflow**
2. Enter tag (e.g., `v0.1.0`) or leave empty for auto-increment
3. Workflow runs tests, creates tag, builds + signs binaries, publishes release

**Required Secrets:** `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`

**Local release testing:**
```bash
task release:dry-run
```
