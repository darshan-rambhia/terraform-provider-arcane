package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ContainerDataSource{}

// NewContainerDataSource returns a new container data source.
func NewContainerDataSource() datasource.DataSource {
	return &ContainerDataSource{}
}

// ContainerDataSource defines the container data source implementation.
type ContainerDataSource struct {
	client *client.Client
}

// ContainerDataSourceModel describes the container data source data model.
type ContainerDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	ProjectID     types.String `tfsdk:"project_id"`
	Name          types.String `tfsdk:"name"`
	Image         types.String `tfsdk:"image"`
	Status        types.String `tfsdk:"status"`
	Health        types.String `tfsdk:"health"`
	Ports         types.List   `tfsdk:"ports"`
}

func (d *ContainerDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (d *ContainerDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use this data source to look up a specific container within an Arcane environment.

Containers can be looked up by ID or by name. When looking up by name, the data source
searches across all projects in the environment (or a specific project if ` + "`project_id`" + ` is set).

## Example Usage

### Lookup by name

` + "```hcl" + `
data "arcane_container" "postgres" {
  environment_id = arcane_environment.production.id
  name           = "postgres"
}

output "postgres_status" {
  value = data.arcane_container.postgres.status
}
` + "```" + `

### Lookup by ID

` + "```hcl" + `
data "arcane_container" "app" {
  environment_id = arcane_environment.production.id
  id             = "abc123"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the container to look up. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment containing the container.",
				Required:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project to filter by. Optional; used to narrow name lookups.",
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the container to look up. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "The image used by the container.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The container status (e.g., running, exited).",
				Computed:            true,
			},
			"health": schema.StringAttribute{
				MarkdownDescription: "The container health check status (healthy, unhealthy, none).",
				Computed:            true,
			},
			"ports": schema.ListNestedAttribute{
				MarkdownDescription: "Port mappings for the container.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_port": schema.Int64Attribute{
							MarkdownDescription: "The port on the host.",
							Computed:            true,
						},
						"container_port": schema.Int64Attribute{
							MarkdownDescription: "The port inside the container.",
							Computed:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "The protocol (tcp, udp).",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ContainerDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContainerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContainerDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := d.client.ForEnvironment(data.EnvironmentID.ValueString())

	var container *client.ContainerDetail

	switch {
	case !data.ID.IsNull() && !data.ID.IsUnknown():
		c, err := envClient.GetContainer(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to get container by ID", err.Error())
			return
		}
		container = c

	case !data.Name.IsNull() && !data.Name.IsUnknown():
		c, err := envClient.GetContainerByName(ctx, data.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to get container by name", err.Error())
			return
		}
		container = c

	default:
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either \"id\" or \"name\" must be specified to look up a container.",
		)
		return
	}

	// Set all fields from the container response
	data.ID = types.StringValue(container.ID)
	data.Name = types.StringValue(container.Name)
	data.Status = types.StringValue(container.Status)

	if container.Image != "" {
		data.Image = types.StringValue(container.Image)
	} else {
		data.Image = types.StringValue("")
	}

	if container.Health != "" {
		data.Health = types.StringValue(container.Health)
	} else {
		data.Health = types.StringValue("")
	}

	// Build ports list
	if len(container.Ports) > 0 {
		portValues := make([]attr.Value, len(container.Ports))
		for i, p := range container.Ports {
			portObj, diags := types.ObjectValue(containerPortObjectType.AttrTypes, map[string]attr.Value{
				"host_port":      types.Int64Value(int64(p.HostPort)),
				"container_port": types.Int64Value(int64(p.ContainerPort)),
				"protocol":       types.StringValue(p.Protocol),
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			portValues[i] = portObj
		}
		var portsDiags diag.Diagnostics
		data.Ports, portsDiags = types.ListValue(containerPortObjectType, portValues)
		resp.Diagnostics.Append(portsDiags...)
	} else {
		emptyList, diags := types.ListValue(containerPortObjectType, []attr.Value{})
		resp.Diagnostics.Append(diags...)
		data.Ports = emptyList
	}

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
