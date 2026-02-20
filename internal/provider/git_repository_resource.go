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
	_ resource.Resource                = &GitRepositoryResource{}
	_ resource.ResourceWithImportState = &GitRepositoryResource{}
)

// NewGitRepositoryResource returns a new git repository resource.
func NewGitRepositoryResource() resource.Resource {
	return &GitRepositoryResource{}
}

// GitRepositoryResource defines the git repository resource implementation.
type GitRepositoryResource struct {
	client *client.Client
}

// GitRepositoryResourceModel describes the git repository resource data model.
type GitRepositoryResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	URL         types.String `tfsdk:"url"`
	Branch      types.String `tfsdk:"branch"`
	AuthType    types.String `tfsdk:"auth_type"`
	Credentials types.String `tfsdk:"credentials"`
}

func (r *GitRepositoryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_git_repository"
}

func (r *GitRepositoryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages an Arcane git repository configuration.

Git repositories in Arcane are used for GitOps workflows. They define the source
repository that Arcane can pull compose files from for automated deployments.

## Example Usage

` + "```hcl" + `
resource "arcane_git_repository" "infra" {
  name        = "homelab-infra"
  url         = "https://github.com/example/homelab-infra.git"
  branch      = "main"
  auth_type   = "token"
  credentials = var.github_token
}

# Use with a GitOps sync
resource "arcane_gitops_sync" "webapp" {
  environment_id = arcane_environment.production.id
  repository_id  = arcane_git_repository.infra.id
  path           = "apps/webapp"
  auto_sync      = true
}
` + "```" + `

## Import

Git repositories can be imported using their ID:

` + "```shell" + `
terraform import arcane_git_repository.infra <repository-id>
` + "```" + `

**Note:** When importing, the credentials field is not retrieved from the API.
You will need to re-specify credentials in your configuration after import.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the git repository.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the git repository. Must be unique.",
				Required:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL of the git repository (e.g., `https://github.com/example/repo.git`).",
				Required:            true,
			},
			"branch": schema.StringAttribute{
				MarkdownDescription: "The branch to use. If not specified, the API may set a default (e.g., `main`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "The authentication type for the repository (e.g., `token`, `ssh`, `basic`).",
				Optional:            true,
			},
			"credentials": schema.StringAttribute{
				MarkdownDescription: "The credentials for repository authentication (e.g., a personal access token). This value is write-only and will not be read back from the API.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *GitRepositoryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GitRepositoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GitRepositoryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.GitRepositoryCreateRequest{
		Name:        data.Name.ValueString(),
		URL:         data.URL.ValueString(),
		Branch:      data.Branch.ValueString(),
		AuthType:    data.AuthType.ValueString(),
		Credentials: data.Credentials.ValueString(),
	}

	repo, err := r.client.CreateGitRepository(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create git repository", err.Error())
		return
	}

	// Update state from response
	data.ID = types.StringValue(repo.ID)
	data.Name = types.StringValue(repo.Name)
	data.URL = types.StringValue(repo.URL)
	if repo.Branch != "" {
		data.Branch = types.StringValue(repo.Branch)
	}
	if repo.AuthType != "" {
		data.AuthType = types.StringValue(repo.AuthType)
	} else if data.AuthType.IsNull() || data.AuthType.ValueString() == "" {
		data.AuthType = types.StringNull()
	}
	// Preserve credentials from plan (API does not return credentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitRepositoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GitRepositoryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	repo, err := r.client.GetGitRepository(ctx, data.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read git repository", err.Error())
		return
	}

	// Update state from response
	data.Name = types.StringValue(repo.Name)
	data.URL = types.StringValue(repo.URL)
	if repo.Branch != "" {
		data.Branch = types.StringValue(repo.Branch)
	}
	if repo.AuthType != "" {
		data.AuthType = types.StringValue(repo.AuthType)
	} else {
		data.AuthType = types.StringNull()
	}
	// Preserve credentials from state (API does not return credentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitRepositoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GitRepositoryResourceModel
	var state GitRepositoryResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.GitRepositoryUpdateRequest{
		Name:        data.Name.ValueString(),
		URL:         data.URL.ValueString(),
		Branch:      data.Branch.ValueString(),
		AuthType:    data.AuthType.ValueString(),
		Credentials: data.Credentials.ValueString(),
	}

	repo, err := r.client.UpdateGitRepository(ctx, data.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update git repository", err.Error())
		return
	}

	// Update state from response
	data.Name = types.StringValue(repo.Name)
	data.URL = types.StringValue(repo.URL)
	if repo.Branch != "" {
		data.Branch = types.StringValue(repo.Branch)
	}
	if repo.AuthType != "" {
		data.AuthType = types.StringValue(repo.AuthType)
	} else {
		data.AuthType = types.StringNull()
	}
	// Preserve credentials from plan (API does not return credentials)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitRepositoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GitRepositoryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteGitRepository(ctx, data.ID.ValueString())
	if err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete git repository", err.Error())
			return
		}
	}
}

func (r *GitRepositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
