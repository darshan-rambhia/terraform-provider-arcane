package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &GitOpsSyncResource{}
	_ resource.ResourceWithImportState = &GitOpsSyncResource{}
)

// NewGitOpsSyncResource returns a new GitOps sync resource.
func NewGitOpsSyncResource() resource.Resource {
	return &GitOpsSyncResource{}
}

// GitOpsSyncResource defines the GitOps sync resource implementation.
type GitOpsSyncResource struct {
	client *client.Client
}

// GitOpsSyncResourceModel describes the GitOps sync resource data model.
type GitOpsSyncResourceModel struct {
	ID             types.String `tfsdk:"id"`
	EnvironmentID  types.String `tfsdk:"environment_id"`
	RepositoryID   types.String `tfsdk:"repository_id"`
	Path           types.String `tfsdk:"path"`
	Branch         types.String `tfsdk:"branch"`
	ComposeFile    types.String `tfsdk:"compose_file"`
	SyncInterval   types.String `tfsdk:"sync_interval"`
	AutoSync       types.Bool   `tfsdk:"auto_sync"`
	LastSyncAt     types.String `tfsdk:"last_sync_at"`
	LastSyncCommit types.String `tfsdk:"last_sync_commit"`
}

func (r *GitOpsSyncResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_sync"
}

func (r *GitOpsSyncResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages a GitOps sync configuration for an Arcane environment.

A GitOps sync links a git repository to an environment, allowing Arcane to automatically
deploy Docker Compose stacks from a repository. When auto_sync is enabled, Arcane will
periodically pull changes from the repository and redeploy the stack.

## Example Usage

### Basic GitOps Sync

` + "```hcl" + `
resource "arcane_gitops_sync" "webapp" {
  environment_id = arcane_environment.production.id
  repository_id  = arcane_git_repository.infra.id
  path           = "apps/webapp"
  branch         = "main"
  compose_file   = "docker-compose.yml"
  auto_sync      = true
  sync_interval  = "5m"
}
` + "```" + `

### Minimal Configuration

` + "```hcl" + `
resource "arcane_gitops_sync" "webapp" {
  environment_id = arcane_environment.production.id
  repository_id  = arcane_git_repository.infra.id
}
` + "```" + `

## Import

GitOps syncs can be imported using ` + "`environment_id/sync_id`" + `:

` + "```shell" + `
terraform import arcane_gitops_sync.webapp env-id/sync-id
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the GitOps sync.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment to sync to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"repository_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the git repository to sync from.",
				Required:            true,
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "The path within the repository containing the compose file.",
				Optional:            true,
			},
			"branch": schema.StringAttribute{
				MarkdownDescription: "The branch to sync from. Defaults to the repository's default branch.",
				Optional:            true,
				Computed:            true,
			},
			"compose_file": schema.StringAttribute{
				MarkdownDescription: "The name of the compose file to deploy. Defaults to `docker-compose.yml`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("docker-compose.yml"),
			},
			"sync_interval": schema.StringAttribute{
				MarkdownDescription: "How often to check for changes (e.g. `5m`, `1h`). Only used when `auto_sync` is enabled.",
				Optional:            true,
			},
			"auto_sync": schema.BoolAttribute{
				MarkdownDescription: "Whether to automatically sync changes from the repository. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"last_sync_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp of the last successful sync in RFC3339 format.",
				Computed:            true,
			},
			"last_sync_commit": schema.StringAttribute{
				MarkdownDescription: "The commit SHA of the last successful sync.",
				Computed:            true,
			},
		},
	}
}

func (r *GitOpsSyncResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GitOpsSyncResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GitOpsSyncResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	createReq := &client.GitOpsSyncCreateRequest{
		RepositoryID: data.RepositoryID.ValueString(),
		Path:         data.Path.ValueString(),
		Branch:       data.Branch.ValueString(),
		ComposeFile:  data.ComposeFile.ValueString(),
		SyncInterval: data.SyncInterval.ValueString(),
		AutoSync:     data.AutoSync.ValueBool(),
	}

	sync, err := envClient.CreateGitOpsSync(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create GitOps sync", err.Error())
		return
	}

	// Update state
	data.ID = types.StringValue(sync.ID)
	data.EnvironmentID = types.StringValue(data.EnvironmentID.ValueString())
	data.RepositoryID = types.StringValue(sync.RepositoryID)
	if sync.Path != "" {
		data.Path = types.StringValue(sync.Path)
	}
	if sync.Branch != "" {
		data.Branch = types.StringValue(sync.Branch)
	}
	if sync.ComposeFile != "" {
		data.ComposeFile = types.StringValue(sync.ComposeFile)
	}
	if sync.SyncInterval != "" {
		data.SyncInterval = types.StringValue(sync.SyncInterval)
	}
	data.AutoSync = types.BoolValue(sync.AutoSync)
	if sync.LastSyncAt != "" {
		data.LastSyncAt = types.StringValue(sync.LastSyncAt)
	} else {
		data.LastSyncAt = types.StringNull()
	}
	if sync.LastSyncCommit != "" {
		data.LastSyncCommit = types.StringValue(sync.LastSyncCommit)
	} else {
		data.LastSyncCommit = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitOpsSyncResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GitOpsSyncResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	sync, err := envClient.GetGitOpsSync(ctx, data.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read GitOps sync", err.Error())
		return
	}

	// Update state from response
	data.RepositoryID = types.StringValue(sync.RepositoryID)
	if sync.Path != "" {
		data.Path = types.StringValue(sync.Path)
	}
	if sync.Branch != "" {
		data.Branch = types.StringValue(sync.Branch)
	}
	if sync.ComposeFile != "" {
		data.ComposeFile = types.StringValue(sync.ComposeFile)
	}
	if sync.SyncInterval != "" {
		data.SyncInterval = types.StringValue(sync.SyncInterval)
	}
	data.AutoSync = types.BoolValue(sync.AutoSync)
	if sync.LastSyncAt != "" {
		data.LastSyncAt = types.StringValue(sync.LastSyncAt)
	} else {
		data.LastSyncAt = types.StringNull()
	}
	if sync.LastSyncCommit != "" {
		data.LastSyncCommit = types.StringValue(sync.LastSyncCommit)
	} else {
		data.LastSyncCommit = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitOpsSyncResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GitOpsSyncResourceModel
	var state GitOpsSyncResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	autoSync := data.AutoSync.ValueBool()
	updateReq := &client.GitOpsSyncUpdateRequest{
		RepositoryID: data.RepositoryID.ValueString(),
		Path:         data.Path.ValueString(),
		Branch:       data.Branch.ValueString(),
		ComposeFile:  data.ComposeFile.ValueString(),
		SyncInterval: data.SyncInterval.ValueString(),
		AutoSync:     &autoSync,
	}

	sync, err := envClient.UpdateGitOpsSync(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update GitOps sync", err.Error())
		return
	}

	// Update computed fields from response
	data.ID = state.ID
	if sync.Branch != "" {
		data.Branch = types.StringValue(sync.Branch)
	}
	if sync.ComposeFile != "" {
		data.ComposeFile = types.StringValue(sync.ComposeFile)
	}
	data.AutoSync = types.BoolValue(sync.AutoSync)
	if sync.LastSyncAt != "" {
		data.LastSyncAt = types.StringValue(sync.LastSyncAt)
	} else {
		data.LastSyncAt = types.StringNull()
	}
	if sync.LastSyncCommit != "" {
		data.LastSyncCommit = types.StringValue(sync.LastSyncCommit)
	} else {
		data.LastSyncCommit = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GitOpsSyncResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GitOpsSyncResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	err := envClient.DeleteGitOpsSync(ctx, data.ID.ValueString())
	if err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete GitOps sync", err.Error())
			return
		}
	}
}

func (r *GitOpsSyncResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: environment_id/sync_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
}
