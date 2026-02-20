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
var _ datasource.DataSource = &EnvironmentHealthDataSource{}

// NewEnvironmentHealthDataSource returns a new environment health data source.
func NewEnvironmentHealthDataSource() datasource.DataSource {
	return &EnvironmentHealthDataSource{}
}

// EnvironmentHealthDataSource defines the environment health data source implementation.
type EnvironmentHealthDataSource struct {
	client *client.Client
}

// EnvironmentHealthDataSourceModel describes the data model.
type EnvironmentHealthDataSourceModel struct {
	EnvironmentID types.String `tfsdk:"environment_id"`
	IsConnected   types.Bool   `tfsdk:"is_connected"`
	ErrorMessage  types.String `tfsdk:"error_message"`
}

func (d *EnvironmentHealthDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_health"
}

func (d *EnvironmentHealthDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use this data source to check whether an Arcane environment's agent is connected and healthy.

This calls the environment test endpoint to verify connectivity. Can be used in preconditions
to ensure the agent is online before attempting deployments.

## Example Usage

` + "```hcl" + `
data "arcane_environment_health" "production" {
  environment_id = arcane_environment.production.id
}

resource "arcane_project_deployment" "webapp" {
  # ...

  lifecycle {
    precondition {
      condition     = data.arcane_environment_health.production.is_connected
      error_message = "Agent is not connected"
    }
  }
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to check.",
				Required:            true,
			},
			"is_connected": schema.BoolAttribute{
				MarkdownDescription: "Whether the agent is connected and responding.",
				Computed:            true,
			},
			"error_message": schema.StringAttribute{
				MarkdownDescription: "Error message if the agent is not connected. Empty when connected.",
				Computed:            true,
			},
		},
	}
}

func (d *EnvironmentHealthDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *EnvironmentHealthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnvironmentHealthDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := d.client.TestEnvironment(ctx, data.EnvironmentID.ValueString())
	if err != nil {
		data.IsConnected = types.BoolValue(false)
		data.ErrorMessage = types.StringValue(err.Error())
	} else {
		data.IsConnected = types.BoolValue(true)
		data.ErrorMessage = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
