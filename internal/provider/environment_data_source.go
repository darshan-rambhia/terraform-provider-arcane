package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EnvironmentDataSource{}

// NewEnvironmentDataSource returns a new environment data source.
func NewEnvironmentDataSource() datasource.DataSource {
	return &EnvironmentDataSource{}
}

// EnvironmentDataSource defines the environment data source implementation.
type EnvironmentDataSource struct {
	client *client.Client
}

// EnvironmentDataSourceModel describes the environment data source data model.
type EnvironmentDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	UseAPIKey   types.Bool   `tfsdk:"use_api_key"`
}

func (d *EnvironmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *EnvironmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use this data source to get information about an existing Arcane environment.

You can look up an environment by either its ID or name.

## Example Usage

### By ID

` + "```hcl" + `
data "arcane_environment" "example" {
  id = "env-123"
}
` + "```" + `

### By Name

` + "```hcl" + `
data "arcane_environment" "example" {
  name = "production"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the environment. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the environment. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the environment.",
				Computed:            true,
			},
			"use_api_key": schema.BoolAttribute{
				MarkdownDescription: "Whether the environment requires API key authentication.",
				Computed:            true,
			},
		},
	}
}

func (d *EnvironmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *EnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that either id or name is specified
	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be specified to look up an environment.",
		)
		return
	}

	var env *client.Environment
	var err error

	if !data.ID.IsNull() {
		// Look up by ID
		env, err = d.client.GetEnvironment(ctx, data.ID.ValueString())
	} else {
		// Look up by name
		env, err = d.client.GetEnvironmentByName(ctx, data.Name.ValueString())
	}

	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}

	// Update state
	data.ID = types.StringValue(env.ID)
	data.Name = types.StringValue(env.Name)
	if env.Description != "" {
		data.Description = types.StringValue(env.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.UseAPIKey = types.BoolValue(env.UseAPIKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
