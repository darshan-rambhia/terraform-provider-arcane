package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// accessTokenPlanModifier handles access_token plan modification based on regenerate_access_token.
type accessTokenPlanModifier struct{}

func (m accessTokenPlanModifier) Description(ctx context.Context) string {
	return "Marks access_token as unknown when regenerate_access_token changes to true"
}

func (m accessTokenPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m accessTokenPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Check if regenerate_access_token is being set to true
	var regenerate types.Bool
	diags := req.Plan.GetAttribute(ctx, path.Root("regenerate_access_token"), &regenerate)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var stateRegenerate types.Bool
	diags = req.State.GetAttribute(ctx, path.Root("regenerate_access_token"), &stateRegenerate)
	resp.Diagnostics.Append(diags...)
	// Ignore errors for state - may not exist yet

	// If regenerate is being set to true (and wasn't already true), mark access_token as unknown
	if regenerate.ValueBool() && !stateRegenerate.ValueBool() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	// For existing resources, preserve the state value if plan is unknown or null.
	// For new resources (state is null), keep the value as unknown so the provider
	// can set it after create without causing an inconsistency error.
	if (req.PlanValue.IsUnknown() || req.PlanValue.IsNull()) && !req.StateValue.IsNull() {
		resp.PlanValue = req.StateValue
	}
}

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &EnvironmentResource{}
	_ resource.ResourceWithImportState = &EnvironmentResource{}
)

// NewEnvironmentResource returns a new environment resource.
func NewEnvironmentResource() resource.Resource {
	return &EnvironmentResource{}
}

// EnvironmentResource defines the environment resource implementation.
type EnvironmentResource struct {
	client *client.Client
}

// EnvironmentResourceModel describes the environment resource data model.
type EnvironmentResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	APIURL                types.String `tfsdk:"api_url"`
	Description           types.String `tfsdk:"description"`
	UseAPIKey             types.Bool   `tfsdk:"use_api_key"`
	AccessToken           types.String `tfsdk:"access_token"`
	RegenerateAccessToken types.Bool   `tfsdk:"regenerate_access_token"`
}

func (r *EnvironmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *EnvironmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages an Arcane environment.

Environments in Arcane represent Docker daemon connections. Each environment can
contain multiple projects (docker-compose stacks). When created, an API key (access token)
is automatically generated for agent authentication.

## Example Usage

` + "```hcl" + `
resource "arcane_environment" "production" {
  name        = "production"
  api_url     = "http://10.100.1.100:3553"
  description = "Production Docker environment"
}

# Access the generated access token for agent authentication
output "env_token" {
  value     = arcane_environment.production.access_token
  sensitive = true
}

# Use the token in your sidecars module
module "sidecars" {
  source = "../../modules/lxc-sidecars"
  # ...
  arcane_agent = {
    enabled      = true
    access_token = arcane_environment.production.access_token
  }
}
` + "```" + `

## Token Rotation

To rotate the access token, set ` + "`regenerate_access_token = true`" + `:

` + "```hcl" + `
resource "arcane_environment" "production" {
  name                    = "production"
  regenerate_access_token = true  # Will regenerate on next apply
}
` + "```" + `

After apply, the new token will be in ` + "`access_token`" + ` and you should set
` + "`regenerate_access_token`" + ` back to ` + "`false`" + `.

## Import

Environments can be imported using their ID:

` + "```shell" + `
terraform import arcane_environment.production <environment-id>
` + "```" + `

**Note:** When importing, the access token is not retrieved from the API. You'll need
to either regenerate it using ` + "`regenerate_access_token = true`" + ` or provide a
fallback token from 1Password.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the environment.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the environment. Must be unique.",
				Required:            true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "The URL where the agent will be accessible (e.g., `http://10.100.2.203:3553`). The manager connects to this URL to communicate with the agent.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "A description of the environment.",
				Optional:            true,
			},
			"use_api_key": schema.BoolAttribute{
				MarkdownDescription: "Whether to require API key authentication for this environment. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "The access token (API key) for this environment. This token has an `arc_` prefix and is used by agents to authenticate with the Arcane manager. Automatically generated on resource creation.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					accessTokenPlanModifier{},
				},
			},
			"regenerate_access_token": schema.BoolAttribute{
				MarkdownDescription: "Set to `true` to regenerate the access token. The new token will be available in `access_token` after apply. Reset to `false` after regeneration.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *EnvironmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *EnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create the environment
	createReq := &client.EnvironmentCreateRequest{
		Name:        data.Name.ValueString(),
		APIURL:      data.APIURL.ValueString(),
		Description: data.Description.ValueString(),
		UseAPIKey:   data.UseAPIKey.ValueBool(),
	}

	env, err := r.client.CreateEnvironment(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment", err.Error())
		return
	}

	// Automatically regenerate the API key to get a valid arc_ prefixed token
	// This is required for agents to authenticate with the manager
	envWithKey, err := r.client.RegenerateEnvironmentAPIKey(ctx, env.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to generate API key for environment", err.Error())
		return
	}

	// Update state
	data.ID = types.StringValue(env.ID)
	data.Name = types.StringValue(env.Name)
	if env.Description != "" {
		data.Description = types.StringValue(env.Description)
	}
	data.UseAPIKey = types.BoolValue(env.UseAPIKey)

	// Use the API key from the regenerate response
	if envWithKey.APIKey != "" {
		data.AccessToken = types.StringValue(envWithKey.APIKey)
	} else if env.AccessToken != "" {
		data.AccessToken = types.StringValue(env.AccessToken)
	} else {
		data.AccessToken = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the environment
	env, err := r.client.GetEnvironment(ctx, data.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}

	// Update state
	data.Name = types.StringValue(env.Name)
	if env.APIURL != "" {
		data.APIURL = types.StringValue(env.APIURL)
	}
	if env.Description != "" {
		data.Description = types.StringValue(env.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.UseAPIKey = types.BoolValue(env.UseAPIKey)
	// Note: access_token is typically not returned on read operations
	// Keep the existing value from state

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EnvironmentResourceModel
	var state EnvironmentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we need to regenerate the access token
	// Note: regenerate_access_token stays true until user sets it back to false
	if data.RegenerateAccessToken.ValueBool() && !state.RegenerateAccessToken.ValueBool() {
		envWithKey, err := r.client.RegenerateEnvironmentAPIKey(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to regenerate API key", err.Error())
			return
		}
		if envWithKey.APIKey != "" {
			data.AccessToken = types.StringValue(envWithKey.APIKey)
		}
	} else if !data.RegenerateAccessToken.ValueBool() && state.RegenerateAccessToken.ValueBool() {
		// User set it back to false - preserve existing access_token from state
		data.AccessToken = state.AccessToken
	}

	// Update the environment if any fields changed
	updateReq := &client.EnvironmentUpdateRequest{}
	needsUpdate := false

	if !data.Name.Equal(state.Name) {
		name := data.Name.ValueString()
		updateReq.Name = name
		needsUpdate = true
	}

	if !data.Description.Equal(state.Description) {
		desc := data.Description.ValueString()
		updateReq.Description = desc
		needsUpdate = true
	}

	if !data.UseAPIKey.Equal(state.UseAPIKey) {
		useAPIKey := data.UseAPIKey.ValueBool()
		updateReq.UseAPIKey = &useAPIKey
		needsUpdate = true
	}

	if needsUpdate {
		env, err := r.client.UpdateEnvironment(ctx, data.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update environment", err.Error())
			return
		}

		// Update state from response
		data.Name = types.StringValue(env.Name)
		if env.Description != "" {
			data.Description = types.StringValue(env.Description)
		} else {
			data.Description = types.StringNull()
		}
		data.UseAPIKey = types.BoolValue(env.UseAPIKey)
	}

	// Preserve existing access_token if not regenerated
	if data.AccessToken.IsNull() || data.AccessToken.IsUnknown() {
		data.AccessToken = state.AccessToken
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EnvironmentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteEnvironment(ctx, data.ID.ValueString())
	if err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Failed to delete environment", err.Error())
			return
		}
	}
}

func (r *EnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
