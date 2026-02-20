package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ProjectDataSource{}

// NewProjectDataSource returns a new project data source.
func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource defines the project data source implementation.
type ProjectDataSource struct {
	client *client.Client
}

// ProjectDataSourceModel describes the project data source data model.
type ProjectDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.String `tfsdk:"environment_id"`
	Name          types.String `tfsdk:"name"`
	Status        types.String `tfsdk:"status"`
	Path          types.String `tfsdk:"path"`
	Services      types.List   `tfsdk:"services"`
}

// ProjectServiceModel describes a service within a project.
type ProjectServiceModel struct {
	Name   types.String `tfsdk:"name"`
	Status types.String `tfsdk:"status"`
	Image  types.String `tfsdk:"image"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use this data source to get information about an existing Arcane project.

Projects in Arcane represent Docker Compose stacks discovered within an environment.
You can look up a project by either its ID or name within an environment.

## Example Usage

### By ID

` + "```hcl" + `
data "arcane_project" "webapp" {
  environment_id = arcane_environment.production.id
  id             = "project-123"
}
` + "```" + `

### By Name

` + "```hcl" + `
data "arcane_project" "webapp" {
  environment_id = arcane_environment.production.id
  name           = "webapp"
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the project. Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment containing the project.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the project (docker-compose stack name). Either `id` or `name` must be specified.",
				Optional:            true,
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The current status of the project (e.g., `running`, `exited`).",
				Computed:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path to the docker-compose file on the Docker host.",
				Computed:            true,
			},
			"services": schema.ListNestedAttribute{
				MarkdownDescription: "The services defined in the project.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the service.",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "The current status of the service.",
							Computed:            true,
						},
						"image": schema.StringAttribute{
							MarkdownDescription: "The image used by the service.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that either id or name is specified
	if data.ID.IsNull() && data.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing Required Attribute",
			"Either 'id' or 'name' must be specified to look up a project.",
		)
		return
	}

	envClient := d.client.ForEnvironment(data.EnvironmentID.ValueString())

	var project *client.Project
	var err error

	if !data.ID.IsNull() {
		// Look up by ID
		project, err = envClient.GetProject(ctx, data.ID.ValueString())
	} else {
		// Look up by name
		project, err = envClient.GetProjectByName(ctx, data.Name.ValueString())
	}

	if err != nil {
		resp.Diagnostics.AddError("Failed to read project", err.Error())
		return
	}

	// Update state
	data.ID = types.StringValue(project.ID)
	data.Name = types.StringValue(project.Name)
	data.Status = types.StringValue(project.Status)

	if project.Path != "" {
		data.Path = types.StringValue(project.Path)
	} else {
		data.Path = types.StringNull()
	}

	// Convert services to list
	serviceObjectType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":   types.StringType,
			"status": types.StringType,
			"image":  types.StringType,
		},
	}

	if len(project.Services) > 0 {
		serviceValues := make([]attr.Value, len(project.Services))
		for i, svc := range project.Services {
			var imageVal attr.Value
			if svc.Image != "" {
				imageVal = types.StringValue(svc.Image)
			} else {
				imageVal = types.StringNull()
			}

			objVal, diags := types.ObjectValue(serviceObjectType.AttrTypes, map[string]attr.Value{
				"name":   types.StringValue(svc.Name),
				"status": types.StringValue(svc.Status),
				"image":  imageVal,
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			serviceValues[i] = objVal
		}

		servicesList, diags := types.ListValue(serviceObjectType, serviceValues)
		resp.Diagnostics.Append(diags...)
		if !resp.Diagnostics.HasError() {
			data.Services = servicesList
		}
	} else {
		data.Services = types.ListNull(serviceObjectType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
