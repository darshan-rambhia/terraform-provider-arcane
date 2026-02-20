package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &ProjectDeploymentResource{}
	_ resource.ResourceWithImportState = &ProjectDeploymentResource{}
)

// lastDeployedAtPlanModifier marks last_deployed_at as unknown when any mutable
// attribute changes (triggers, pull, force_recreate, remove_orphans), since the
// Update method will set it to time.Now(). When nothing changes, it preserves
// the state value. This prevents "Provider produced inconsistent result" errors.
type lastDeployedAtPlanModifier struct{}

func (m lastDeployedAtPlanModifier) Description(ctx context.Context) string {
	return "Marks last_deployed_at as unknown when deployment-triggering attributes change"
}

func (m lastDeployedAtPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m lastDeployedAtPlanModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// On create (no state yet), keep as unknown so provider can set it
	if req.StateValue.IsNull() {
		return
	}

	// Check if any deployment-triggering attribute changed
	changed := false

	// Check triggers
	var planTriggers, stateTriggers types.Map
	req.Plan.GetAttribute(ctx, path.Root("triggers"), &planTriggers)
	req.State.GetAttribute(ctx, path.Root("triggers"), &stateTriggers)
	if !planTriggers.Equal(stateTriggers) {
		changed = true
	}

	// Check bool options
	for _, attr := range []string{"pull", "force_recreate", "remove_orphans"} {
		var planVal, stateVal types.Bool
		req.Plan.GetAttribute(ctx, path.Root(attr), &planVal)
		req.State.GetAttribute(ctx, path.Root(attr), &stateVal)
		if !planVal.Equal(stateVal) {
			changed = true
			break
		}
	}

	if changed {
		resp.PlanValue = types.StringUnknown()
	} else {
		// Nothing changed — preserve the current state value
		resp.PlanValue = req.StateValue
	}
}

// NewProjectDeploymentResource returns a new project deployment resource.
func NewProjectDeploymentResource() resource.Resource {
	return &ProjectDeploymentResource{}
}

// ProjectDeploymentResource defines the project deployment resource implementation.
type ProjectDeploymentResource struct {
	client *client.Client
}

// ProjectDeploymentResourceModel describes the project deployment resource data model.
type ProjectDeploymentResourceModel struct {
	ID             types.String `tfsdk:"id"`
	EnvironmentID  types.String `tfsdk:"environment_id"`
	ProjectID      types.String `tfsdk:"project_id"`
	Pull           types.Bool   `tfsdk:"pull"`
	ForceRecreate  types.Bool   `tfsdk:"force_recreate"`
	RemoveOrphans  types.Bool   `tfsdk:"remove_orphans"`
	StopOnDelete   types.Bool   `tfsdk:"stop_on_delete"`
	Triggers       types.Map    `tfsdk:"triggers"`
	WaitTimeout    types.String `tfsdk:"wait_timeout"`
	Status         types.String `tfsdk:"status"`
	LastDeployedAt types.String `tfsdk:"last_deployed_at"`
}

func (r *ProjectDeploymentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_deployment"
}

func (r *ProjectDeploymentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages the deployment state of an Arcane project.

This resource triggers deployment operations (up/redeploy) for Docker Compose projects
in Arcane. It tracks the deployment state and can be used to ensure projects are running.

## Behavior

- **Create**: Calls the project's deploy (up) endpoint to start the stack
- **Update**: Calls the project's redeploy endpoint when triggers or options change
- **Delete**: Behavior depends on ` + "`stop_on_delete`" + `:
  - ` + "`false`" + ` (default): Removes from Terraform state only, containers continue running
  - ` + "`true`" + `: Stops containers (docker compose down) before removing from state
- **Read**: Fetches the current project status

## Example Usage

### Basic Deployment

` + "```hcl" + `
resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id
}
` + "```" + `

### With Triggers (Recommended)

` + "```hcl" + `
resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id

  # Only redeploy when compose or env files change
  triggers = {
    compose = sha256(file("deploy/docker-compose.yml"))
    env     = sha256(local.env_content)
  }

  pull           = true
  force_recreate = false
  remove_orphans = true
  stop_on_delete = true
}
` + "```" + `

### With Wait Timeout

` + "```hcl" + `
resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id

  # Wait up to 3 minutes for agent to come online
  wait_timeout = "3m"
}
` + "```" + `

## Triggering Redeployments

To force a redeployment, you can use Terraform's replace functionality:

` + "```shell" + `
terraform apply -replace="arcane_project_deployment.webapp"
` + "```" + `

## Import

Deployments can be imported using ` + "`environment_id/project_id`" + `:

` + "```shell" + `
terraform import arcane_project_deployment.webapp env-id/project-id
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier for this deployment (environment_id/project_id).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"environment_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the environment containing the project.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the project to deploy.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pull": schema.BoolAttribute{
				MarkdownDescription: "Pull images before deploying. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"force_recreate": schema.BoolAttribute{
				MarkdownDescription: "Force recreate containers even if configuration hasn't changed. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"remove_orphans": schema.BoolAttribute{
				MarkdownDescription: "Remove containers for services not defined in the compose file. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"stop_on_delete": schema.BoolAttribute{
				MarkdownDescription: "Stop containers (docker compose down) when this resource is destroyed. Defaults to `false`. Set to `false` for projects containing the Arcane agent to prevent self-destruction.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"triggers": schema.MapAttribute{
				MarkdownDescription: "A map of arbitrary strings that, when changed, will trigger a redeployment. Use this to redeploy only when specific files change, e.g. `{ compose = sha256(file(\"docker-compose.yml\")) }`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"wait_timeout": schema.StringAttribute{
				MarkdownDescription: "How long to wait for the agent to come online before deploying. Accepts Go duration strings (e.g. `30s`, `2m`, `5m`). Defaults to `2m`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("2m"),
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The current status of the project.",
				Computed:            true,
			},
			"last_deployed_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp of the last deployment in RFC3339 format.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					lastDeployedAtPlanModifier{},
				},
			},
		},
	}
}

func (r *ProjectDeploymentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// waitForAgent waits for the agent to be reachable by polling the project endpoint.
func (r *ProjectDeploymentResource) waitForAgent(ctx context.Context, envClient *client.EnvironmentClient, projectID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 5 * time.Second

	for {
		_, err := envClient.GetProject(ctx, projectID)
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for agent after %s: %w", timeout, err)
		}

		tflog.Debug(ctx, "Agent not ready, retrying", map[string]interface{}{
			"backoff":    backoff.String(),
			"project_id": projectID,
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Cap backoff at 30s
		if backoff < 30*time.Second {
			backoff = backoff * 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (r *ProjectDeploymentResource) parseWaitTimeout(data *ProjectDeploymentResourceModel) time.Duration {
	timeoutStr := data.WaitTimeout.ValueString()
	if timeoutStr == "" {
		return 2 * time.Minute
	}
	d, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return 2 * time.Minute
	}
	return d
}

func (r *ProjectDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectDeploymentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	// Wait for agent to be reachable
	timeout := r.parseWaitTimeout(&data)
	if err := r.waitForAgent(ctx, envClient, data.ProjectID.ValueString(), timeout); err != nil {
		resp.Diagnostics.AddError("Agent not reachable", err.Error())
		return
	}

	// Deploy the project
	deployReq := &client.ProjectDeployRequest{
		Pull:          data.Pull.ValueBool(),
		ForceRecreate: data.ForceRecreate.ValueBool(),
		RemoveOrphans: data.RemoveOrphans.ValueBool(),
	}

	tflog.Debug(ctx, "Deploying project", map[string]interface{}{
		"environment_id": data.EnvironmentID.ValueString(),
		"project_id":     data.ProjectID.ValueString(),
	})

	err := envClient.DeployProject(ctx, data.ProjectID.ValueString(), deployReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to deploy project", err.Error())
		return
	}

	// Get current project status
	project, err := envClient.GetProject(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get project status", err.Error())
		return
	}

	// Update state
	data.ID = types.StringValue(fmt.Sprintf("%s/%s", data.EnvironmentID.ValueString(), data.ProjectID.ValueString()))
	data.Status = types.StringValue(project.Status)
	data.LastDeployedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectDeploymentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	// Get current project status
	project, err := envClient.GetProject(ctx, data.ProjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to get project status", err.Error())
		return
	}

	// Update status only - triggers and last_deployed_at are preserved from state
	data.Status = types.StringValue(project.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectDeploymentResourceModel
	var state ProjectDeploymentResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

	// Redeploy the project
	deployReq := &client.ProjectDeployRequest{
		Pull:          data.Pull.ValueBool(),
		ForceRecreate: data.ForceRecreate.ValueBool(),
		RemoveOrphans: data.RemoveOrphans.ValueBool(),
	}

	tflog.Debug(ctx, "Redeploying project", map[string]interface{}{
		"environment_id": data.EnvironmentID.ValueString(),
		"project_id":     data.ProjectID.ValueString(),
	})

	err := envClient.RedeployProject(ctx, data.ProjectID.ValueString(), deployReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to redeploy project", err.Error())
		return
	}

	// Get current project status
	project, err := envClient.GetProject(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get project status", err.Error())
		return
	}

	// Update state
	data.Status = types.StringValue(project.Status)
	data.LastDeployedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectDeploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectDeploymentResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should stop containers on delete
	if data.StopOnDelete.ValueBool() {
		envClient := r.client.ForEnvironment(data.EnvironmentID.ValueString())

		tflog.Info(ctx, "Stopping project (stop_on_delete=true)", map[string]interface{}{
			"environment_id": data.EnvironmentID.ValueString(),
			"project_id":     data.ProjectID.ValueString(),
		})

		err := envClient.StopProject(ctx, data.ProjectID.ValueString())
		if err != nil {
			if !client.IsNotFound(err) {
				resp.Diagnostics.AddError("Failed to stop project", err.Error())
				return
			}
		}
	} else {
		// Default: just remove from state, keep containers running
		tflog.Info(ctx, "Removing deployment from state (containers will continue running, stop_on_delete=false)", map[string]interface{}{
			"environment_id": data.EnvironmentID.ValueString(),
			"project_id":     data.ProjectID.ValueString(),
		})
	}
}

func (r *ProjectDeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected format: environment_id/project_id, got: %s", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
}
