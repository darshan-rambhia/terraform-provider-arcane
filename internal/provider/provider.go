package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure ArcaneProvider satisfies provider interfaces.
var _ provider.Provider = &ArcaneProvider{}

// ArcaneProvider defines the provider implementation.
type ArcaneProvider struct {
	version string
}

// ArcaneProviderModel describes the provider data model.
type ArcaneProviderModel struct {
	URL    types.String `tfsdk:"url"`
	APIKey types.String `tfsdk:"api_key"`
}

// New returns a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ArcaneProvider{
			version: version,
		}
	}
}

func (p *ArcaneProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "arcane"
	resp.Version = p.version
}

func (p *ArcaneProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
The Arcane provider manages resources in [Arcane](https://github.com/darshan-raul/arcane),
a container management platform that provides a unified API for Docker environments.

## Authentication

The provider requires an API URL and optionally an API key for authentication:

- **url**: The Arcane API URL (e.g., ` + "`http://arcane.local:8000`" + `)
- **api_key**: Optional API key for authentication

These can also be set via environment variables:
- ` + "`ARCANE_URL`" + `
- ` + "`ARCANE_API_KEY`" + `

## Example Usage

` + "```hcl" + `
provider "arcane" {
  url     = "http://arcane.homelab.local:8000"
  api_key = var.arcane_api_key
}

# Create an environment
resource "arcane_environment" "production" {
  name        = "production"
  description = "Production environment"
  use_api_key = true
}

# Look up an existing project
data "arcane_project" "webapp" {
  environment_id = arcane_environment.production.id
  name           = "webapp"
}

# Deploy the project
resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id
  pull           = true
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "The Arcane API URL (e.g., `http://arcane.local:8000`). Can also be set via the `ARCANE_URL` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The Arcane API key for authentication. Can also be set via the `ARCANE_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *ArcaneProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ArcaneProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get URL from config or environment
	url := config.URL.ValueString()
	if url == "" {
		url = os.Getenv("ARCANE_URL")
	}
	if url == "" {
		resp.Diagnostics.AddError(
			"Missing Arcane URL",
			"The provider requires an Arcane URL. Set it in the provider configuration or via the ARCANE_URL environment variable.",
		)
		return
	}

	// Get API key from config or environment
	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("ARCANE_API_KEY")
	}

	// Create client
	c, err := client.New(client.Config{
		URL:    url,
		APIKey: apiKey,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create Arcane client",
			err.Error(),
		)
		return
	}

	// Make client available to resources and data sources
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *ArcaneProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEnvironmentResource,
		NewProjectDeploymentResource,
		NewContainerRegistryResource,
		NewGitRepositoryResource,
		NewGitOpsSyncResource,
	}
}

func (p *ArcaneProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentDataSource,
		NewProjectDataSource,
		NewProjectStatusDataSource,
		NewEnvironmentHealthDataSource,
		NewContainerDataSource,
	}
}
