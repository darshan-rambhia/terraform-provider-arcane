# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-20

### Added

**Resources:**
- `arcane_environment` - Manage Arcane environments (Docker daemon connections) with auto-generated API key authentication
- `arcane_project_deployment` - Manage project lifecycle (deploy/undeploy) with content hash-based change detection
- `arcane_container_registry` - Manage container registry configurations
- `arcane_git_repository` - Manage Git repository configurations for GitOps workflows
- `arcane_gitops_sync` - Manage GitOps sync configurations

**Data Sources:**
- `arcane_environment` - Look up environments by ID or name
- `arcane_project` - Look up projects within an environment
- `arcane_project_status` - Get container-level details (health, ports, container IDs)
- `arcane_environment_health` - Check agent connectivity and environment health
- `arcane_container` - Look up individual containers

**Provider Features:**
- API key authentication with automatic token generation on environment creation
- Import support for all resources
- Configurable wait timeout for agent connectivity checks
- RFC3339 timestamp tracking for deployment history
- Multi-platform builds: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, freebsd/arm64, linux/arm, linux/386
- GPG-signed release artifacts with SHA256 checksums
- Terraform Registry compatible with protocol version 6.0
