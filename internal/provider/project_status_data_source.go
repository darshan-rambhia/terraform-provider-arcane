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
var _ datasource.DataSource = &ProjectStatusDataSource{}

// NewProjectStatusDataSource returns a new project status data source.
func NewProjectStatusDataSource() datasource.DataSource {
	return &ProjectStatusDataSource{}
}

// ProjectStatusDataSource defines the project status data source implementation.
type ProjectStatusDataSource struct {
	client *client.Client
}

// ProjectStatusDataSourceModel describes the project status data source data model.
type ProjectStatusDataSourceModel struct {
	EnvironmentID types.String `tfsdk:"environment_id"`
	ProjectID     types.String `tfsdk:"project_id"`
	Name          types.String `tfsdk:"name"`
	Status        types.String `tfsdk:"status"`
	Path          types.String `tfsdk:"path"`
	Containers    types.List   `tfsdk:"containers"`
}

func (d *ProjectStatusDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_status"
}

func (d *ProjectStatusDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use this data source to query detailed runtime status of an Arcane project's containers.

Unlike the ` + "`arcane_project`" + ` data source which returns basic service info, this data source
provides container-level details including health checks, port mappings, and container IDs.

## Example Usage

` + "```hcl" + `
data "arcane_project_status" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id
}

output "container_health" {
  value = data.arcane_project_status.webapp.containers
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment containing the project.",
				Required:            true,
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project to query.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the project.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The overall project status.",
				Computed:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path to the docker-compose file on the host.",
				Computed:            true,
			},
			"containers": schema.ListNestedAttribute{
				MarkdownDescription: "The containers in this project with detailed runtime information.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The container ID.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The container name.",
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
				},
			},
		},
	}
}

func (d *ProjectStatusDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

var containerPortObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"host_port":      types.Int64Type,
		"container_port": types.Int64Type,
		"protocol":       types.StringType,
	},
}

var containerObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":     types.StringType,
		"name":   types.StringType,
		"image":  types.StringType,
		"status": types.StringType,
		"health": types.StringType,
		"ports":  types.ListType{ElemType: containerPortObjectType},
	},
}

func (d *ProjectStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectStatusDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := d.client.ForEnvironment(data.EnvironmentID.ValueString())

	// Get project with container details
	project, err := envClient.GetProject(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read project status", err.Error())
		return
	}

	data.Name = types.StringValue(project.Name)
	data.Status = types.StringValue(project.Status)

	if project.Path != "" {
		data.Path = types.StringValue(project.Path)
	} else {
		data.Path = types.StringNull()
	}

	// Get container details
	containers, err := envClient.GetProjectContainers(ctx, data.ProjectID.ValueString())
	if err != nil {
		// Fallback: build container list from project services
		if len(project.Services) > 0 {
			containerValues := make([]attr.Value, len(project.Services))
			for i, svc := range project.Services {
				portsListVal, diags := types.ListValue(containerPortObjectType, []attr.Value{})
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}

				objVal, diags := types.ObjectValue(containerObjectType.AttrTypes, map[string]attr.Value{
					"id":     types.StringValue(""),
					"name":   types.StringValue(svc.Name),
					"image":  types.StringValue(svc.Image),
					"status": types.StringValue(svc.Status),
					"health": types.StringValue(""),
					"ports":  portsListVal,
				})
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				containerValues[i] = objVal
			}
			containerList, diags := types.ListValue(containerObjectType, containerValues)
			resp.Diagnostics.Append(diags...)
			if !resp.Diagnostics.HasError() {
				data.Containers = containerList
			}
		} else {
			data.Containers = types.ListNull(containerObjectType)
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build container list from detailed response
	if len(containers) > 0 {
		containerValues := make([]attr.Value, len(containers))
		for i, c := range containers {
			// Build ports list
			var portsListVal types.List
			if len(c.Ports) > 0 {
				portValues := make([]attr.Value, len(c.Ports))
				for j, p := range c.Ports {
					portObj, diags := types.ObjectValue(containerPortObjectType.AttrTypes, map[string]attr.Value{
						"host_port":      types.Int64Value(int64(p.HostPort)),
						"container_port": types.Int64Value(int64(p.ContainerPort)),
						"protocol":       types.StringValue(p.Protocol),
					})
					resp.Diagnostics.Append(diags...)
					if resp.Diagnostics.HasError() {
						return
					}
					portValues[j] = portObj
				}
				var portsDiags diag.Diagnostics
				portsListVal, portsDiags = types.ListValue(containerPortObjectType, portValues)
				resp.Diagnostics.Append(portsDiags...)
			} else {
				emptyList, diags := types.ListValue(containerPortObjectType, []attr.Value{})
				resp.Diagnostics.Append(diags...)
				portsListVal = emptyList
			}

			if resp.Diagnostics.HasError() {
				return
			}

			imageVal := types.StringValue("")
			if c.Image != "" {
				imageVal = types.StringValue(c.Image)
			}

			healthVal := types.StringValue("")
			if c.Health != "" {
				healthVal = types.StringValue(c.Health)
			}

			objVal, diags := types.ObjectValue(containerObjectType.AttrTypes, map[string]attr.Value{
				"id":     types.StringValue(c.ID),
				"name":   types.StringValue(c.Name),
				"image":  imageVal,
				"status": types.StringValue(c.Status),
				"health": healthVal,
				"ports":  portsListVal,
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			containerValues[i] = objVal
		}

		containerList, diags := types.ListValue(containerObjectType, containerValues)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			data.Containers = containerList
		}
	} else {
		data.Containers = types.ListNull(containerObjectType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
