# Terraform Provider for Arcane

A Terraform/OpenTofu provider for [Arcane](https://github.com/darshan-raul/arcane), a container management platform that provides a unified API for Docker environments.

## Features

- Manage Arcane environments (Docker daemon connections)
- Look up and deploy projects (Docker Compose stacks)
- Support for API key authentication
- Auto-generated from Arcane's OpenAPI specification

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0 or [OpenTofu](https://opentofu.org/) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22 (for building)
- A running [Arcane](https://github.com/darshan-raul/arcane) instance

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/darshan-raul/terraform-provider-arcane.git
cd terraform-provider-arcane

# Build and install locally
make install
```

### Development Override

For local development, add this to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "darshan-raul/arcane" = "/path/to/terraform-provider-arcane"
  }
  direct {}
}
```

## Usage

### Provider Configuration

```hcl
terraform {
  required_providers {
    arcane = {
      source  = "darshan-raul/arcane"
      version = "~> 0.1"
    }
  }
}

provider "arcane" {
  url     = "http://arcane.homelab.local:8000"
  api_key = var.arcane_api_key  # Optional
}
```

### Environment Variables

- `ARCANE_URL` - Arcane API URL
- `ARCANE_API_KEY` - API key for authentication

### Resources

#### arcane_environment

Manages an Arcane environment (Docker daemon connection).

```hcl
resource "arcane_environment" "production" {
  name        = "production"
  description = "Production Docker environment"
  use_api_key = true
}
```

#### arcane_project_deployment

Manages the deployment state of a project.

```hcl
resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id

  pull           = true
  force_recreate = false
  remove_orphans = false
}
```

### Data Sources

#### arcane_environment

Look up an existing environment by ID or name.

```hcl
data "arcane_environment" "production" {
  name = "production"
}
```

#### arcane_project

Look up a project within an environment.

```hcl
data "arcane_project" "webapp" {
  environment_id = data.arcane_environment.production.id
  name           = "webapp"
}
```

## Development

### Building

```bash
make build
```

### Testing

```bash
# Unit tests
make test

# Acceptance tests (requires running Arcane instance)
ARCANE_URL=http://localhost:8000 make testacc
```

### Code Generation

This provider uses HashiCorp's code generation tooling to generate resources from Arcane's OpenAPI specification.

```bash
# Install tools
make install-tools

# Fetch OpenAPI spec and regenerate
ARCANE_URL=http://localhost:8000 make generate
```

## Architecture

```
terraform-provider-arcane/
├── internal/
│   ├── provider/          # Provider and resource implementations
│   └── client/            # HTTP client for Arcane API
├── generator/             # Code generation config and templates
├── spec/                  # OpenAPI specs (generated)
├── examples/              # Usage examples
└── docs/                  # Generated documentation
```

## Roadmap

### Phase 1 (Complete)

- [x] `arcane_environment` resource
- [x] `arcane_environment` data source
- [x] `arcane_project` data source
- [x] `arcane_project_deployment` resource

### Phase 2 (Complete)

- [x] `triggers` attribute on `arcane_project_deployment` (hash-based change detection)
- [x] `last_deployed_at` attribute on `arcane_project_deployment` (RFC3339 timestamp)
- [x] Import support for `arcane_project_deployment` (`environment_id/project_id`)
- [x] Agent wait/retry logic with configurable `wait_timeout`
- [x] `arcane_project_status` data source (container-level details: health, ports, IDs)
- [x] `arcane_environment_health` data source (agent connectivity check)
- [x] Fix `UpdateEnvironment` response parsing (`SingleResponse[Environment]`)

### Phase 3 (Planned)

- [ ] `arcane_container_registry` resource
- [ ] `arcane_gitops_sync` resource
- [ ] `arcane_git_repository` resource

### Phase 4 (Future)

- [ ] Full code generation from OpenAPI
- [ ] Container, network, volume resources
- [ ] Registry publishing
- [ ] Acceptance test CI pipeline

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.
