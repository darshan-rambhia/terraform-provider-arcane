package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ContainerRegistryResource{}
	_ resource.ResourceWithImportState = &ContainerRegistryResource{}
)

// NewContainerRegistryResource returns a new container registry resource.
func NewContainerRegistryResource() resource.Resource {
	return &ContainerRegistryResource{}
}

// ContainerRegistryResource defines the container registry resource implementation.
type ContainerRegistryResource struct {
	client *client.Client
}

// ContainerRegistryResourceModel describes the container registry resource data model.
type ContainerRegistryResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	URL      types.String `tfsdk:"url"`
	AuthType types.String `tfsdk:"auth_type"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (r *ContainerRegistryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container_registry"
}

func (r *ContainerRegistryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages an Arcane container registry.

Container registries in Arcane store credentials for authenticating with Docker registries
when pulling private images. Registries are configured globally and can be used across
all environments.

## Example Usage

` + "```hcl" + `
resource "arcane_container_registry" "ghcr" {
  name      = "GitHub Container Registry"
  url       = "https://ghcr.io"
  auth_type = "basic"
  username  = "my-github-user"
  password  = var.ghcr_token
}

resource "arcane_container_registry" "dockerhub" {
  name = "Docker Hub"
  url  = "https://index.docker.io/v1/"
}
` + "```" + `

## Import

Container registries can be imported using their ID:

` + "```shell" + `
terraform import arcane_container_registry.ghcr <registry-id>
` + "```" + `

**Note:** When importing, the password is not retrieved from the API. You will need to
re-supply the password in your configuration after import.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the container registry.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the container registry. Must be unique.",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL of the container registry (e.g., `https://ghcr.io`, `https://index.docker.io/v1/`).",
				Required:            true,
			},
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "The authentication type for the registry (e.g., `basic`). Leave empty for anonymous access.",
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "The username for registry authentication.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The password or token for registry authentication. This value is write-only and will not be read back from the API.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *ContainerRegistryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *ContainerRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContainerRegistryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.ContainerRegistryCreateRequest{
		Name:     data.Name.ValueString(),
		URL:      data.URL.ValueString(),
		AuthType: data.AuthType.ValueString(),
		Username: data.Username.ValueString(),
		Password: data.Password.ValueString(),
	}

	registry, err := r.client.CreateContainerRegistry(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create container registry", err.Error())
		return
	}

	// Update state from response
	data.ID = types.StringValue(registry.ID)
	data.Name = types.StringValue(registry.Name)
	data.URL = types.StringValue(registry.URL)
	if registry.AuthType != "" {
		data.AuthType = types.StringValue(registry.AuthType)
	}
	if registry.Username != "" {
		data.Username = types.StringValue(registry.Username)
	}
	// Password is write-only; preserve from plan since API won't return it

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContainerRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContainerRegistryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	registry, err := r.client.GetContainerRegistry(ctx, data.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read container registry", err.Error())
		return
	}

	// Update state from response
	data.Name = types.StringValue(registry.Name)
	data.URL = types.StringValue(registry.URL)
	if registry.AuthType != "" {
		data.AuthType = types.StringValue(registry.AuthType)
	} else {
		data.AuthType = types.StringNull()
	}
	if registry.Username != "" {
		data.Username = types.StringValue(registry.Username)
	} else {
		data.Username = types.StringNull()
	}
	// Password is write-only; preserve from state since API won't return it

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContainerRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ContainerRegistryResourceModel
	var state ContainerRegistryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.ContainerRegistryUpdateRequest{
		Name:     data.Name.ValueString(),
		URL:      data.URL.ValueString(),
		AuthType: data.AuthType.ValueString(),
		Username: data.Username.ValueString(),
		Password: data.Password.ValueString(),
	}

	registry, err := r.client.UpdateContainerRegistry(ctx, data.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update container registry", err.Error())
		return
	}

	// Update state from response
	data.Name = types.StringValue(registry.Name)
	data.URL = types.StringValue(registry.URL)
	if registry.AuthType != "" {
		data.AuthType = types.StringValue(registry.AuthType)
	} else {
		data.AuthType = types.StringNull()
	}
	if registry.Username != "" {
		data.Username = types.StringValue(registry.Username)
	} else {
		data.Username = types.StringNull()
	}
	// Password is write-only; preserve from plan since API won't return it

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContainerRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContainerRegistryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteContainerRegistry(ctx, data.ID.ValueString())
	if err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete container registry", err.Error())
			return
		}
	}
}

func (r *ContainerRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
