# Contributing to terraform-provider-arcane

First off, thanks for taking the time to contribute! ❤️

All types of contributions are encouraged and valued. See the [Table of Contents](#table-of-contents) for different ways to help and details about how this project handles them. Please make sure to read the relevant section before making your contribution. It will make it a lot easier for us maintainers and smooth out the experience for all involved. The community looks forward to your contributions.

## Table of Contents

- [I Have a Question](#i-have-a-question)
- [I Want To Contribute](#i-want-to-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Enhancements](#suggesting-enhancements)
  - [Your First Code Contribution](#your-first-code-contribution)
- [Development Workflow](#development-workflow)

## I Have a Question

If you want to ask a question, we assume that you have read the available [Documentation](README.md).

Before you ask a question, it is best to search for existing [Issues](https://github.com/darshan-rambhia/terraform-provider-arcane/issues) that might help you. In case you have found a suitable issue and still need clarification, you can write your question in this issue. It is also advisable to search the internet for answers first.

If you then still feel the need to ask a question and need clarification, we recommend the following:

- Open an [Issue](https://github.com/darshan-rambhia/terraform-provider-arcane/issues/new).
- Provide as much context as you can about what you're running into.
- Provide project and platform versions (Go, Terraform/OpenTofu, provider version), depending on what seems relevant.

## I Want To Contribute

### Reporting Bugs

- Make sure that you are using the latest version.
- Read the [documentation](README.md) to find out if the functionality is supported.
- Check if there is already an existing issue to avoid duplicates.
- Open a new issue using the **Bug Report** template.

### Suggesting Enhancements

- Check if there is already an existing issue to avoid duplicates.
- Open a new issue using the **Feature Request** template.

### Your First Code Contribution

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/amazing-feature`).
3. Make your changes.
4. Run tests (`task test:unit`).
5. Commit your changes (`git commit -m 'Add some amazing feature'`).
6. Push to the branch (`git push origin feature/amazing-feature`).
7. Open a Pull Request.

## Development Workflow

This project uses `Taskfile` for development commands.

### Prerequisites

- Go 1.21+
- Terraform or OpenTofu
- [Task](https://taskfile.dev/) (required for build commands)
- A running [Arcane](https://github.com/darshan-rambhia/arcane) instance (for acceptance tests)

### Common Commands

```bash
# Build the provider
task build

# Install locally for testing
task install

# Run unit tests
task test:unit

# Run linting
task lint

# Generate documentation
task docs
```

### Testing

- **Unit Tests**: Run fast with `task test:unit`. No external dependencies required.
- **Acceptance Tests**: Require a running Arcane instance. Set `TF_ACC=1` to enable.

Please ensure all tests pass before submitting a PR.

## CI/CD Pipelines

### Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `test.yml` | PR, push to main | Build, lint, unit tests, acceptance tests (Terraform + OpenTofu) |
| `lint.yml` | PR, push to main | Dedicated linting for badge |
| `docs.yml` | PR, push (when schema/docs change) | Verify documentation is up to date |
| `release.yml` | Manual dispatch | Create tag and publish release |
| `dependabot-auto-merge.yml` | Dependabot PRs | Auto-merge patch/minor dependency updates |

### Creating a Release

Releases are manual. To create a new release:

1. Go to **Actions** > **Release** > **Run workflow**
2. Either:
   - Enter a specific tag (e.g., `v0.2.0`)
   - Leave empty to auto-increment patch version
3. The workflow will:
   - Run tests
   - Create the git tag
   - Build and sign binaries with GPG
   - Publish to GitHub Releases

Version bump hints (for auto-increment):
- Default: patch bump (`v0.1.0` → `v0.1.1`)
- Include `#minor` in commit message for minor bump
- Include `#major` in commit message for major bump

### Documentation

Documentation is generated from schema using `tfplugindocs`. When you change resource/data source schemas:

1. Run `task docs` locally
2. Commit the updated `docs/` folder
3. CI will verify docs are in sync

### Required Secrets

For maintainers setting up the repository:

| Secret | Purpose |
|--------|---------|
| `GPG_PRIVATE_KEY` | Signs release binaries |
| `GPG_PASSPHRASE` | GPG key passphrase |
| `CODECOV_TOKEN` | Coverage reporting (optional) |
