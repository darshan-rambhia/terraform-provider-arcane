package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// TestProjectDeploymentResource_GivenExistingProject_WhenDeployed_ThenStatusRunning
// validates that deploying a project sets its status to running and last_deployed_at is set.
func TestProjectDeploymentResource_GivenExistingProject_WhenDeployed_ThenStatusRunning(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-basic"] = &client.Environment{
		ID:   "env-basic",
		Name: "basic-env",
	}
	mockServer.HealthyEnvs["env-basic"] = true
	mockServer.AddProject("env-basic", &client.Project{
		ID:            "proj-basic",
		Name:          "basic-project",
		Status:        "stopped",
		EnvironmentID: "env-basic",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDeploymentConfig(mockServer.URL, "env-basic", "proj-basic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "environment_id", "env-basic"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "project_id", "proj-basic"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
					resource.TestCheckResourceAttrSet("arcane_project_deployment.test", "last_deployed_at"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenAllOptions_WhenCreated_ThenAllOptionsSet
// validates that all deployment options (pull, force_recreate, remove_orphans) are correctly set.
func TestProjectDeploymentResource_GivenAllOptions_WhenCreated_ThenAllOptionsSet(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-allopts"] = &client.Environment{
		ID:   "env-allopts",
		Name: "allopts-env",
	}
	mockServer.HealthyEnvs["env-allopts"] = true
	mockServer.AddProject("env-allopts", &client.Project{
		ID:            "proj-allopts",
		Name:          "allopts-project",
		Status:        "stopped",
		EnvironmentID: "env-allopts",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDeploymentConfigAllOptions(mockServer.URL, "env-allopts", "proj-allopts", true, true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "pull", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "force_recreate", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "remove_orphans", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenTriggers_WhenCreated_ThenTriggersStored
// validates that triggers are stored in state on initial creation.
func TestProjectDeploymentResource_GivenTriggers_WhenCreated_ThenTriggersStored(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-triggers"] = &client.Environment{
		ID:   "env-triggers",
		Name: "triggers-env",
	}
	mockServer.HealthyEnvs["env-triggers"] = true
	mockServer.AddProject("env-triggers", &client.Project{
		ID:            "proj-triggers",
		Name:          "triggers-project",
		Status:        "stopped",
		EnvironmentID: "env-triggers",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDeploymentConfigWithTriggers(mockServer.URL, "env-triggers", "proj-triggers", map[string]string{
					"compose": "abc123",
					"env":     "def456",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "abc123"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.env", "def456"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenTriggersChanged_WhenUpdated_ThenRedeployed
// validates that changing trigger values causes a redeployment and updates last_deployed_at.
func TestProjectDeploymentResource_GivenTriggersChanged_WhenUpdated_ThenRedeployed(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-trigupd"] = &client.Environment{
		ID:   "env-trigupd",
		Name: "trigupd-env",
	}
	mockServer.HealthyEnvs["env-trigupd"] = true
	mockServer.AddProject("env-trigupd", &client.Project{
		ID:            "proj-trigupd",
		Name:          "trigupd-project",
		Status:        "stopped",
		EnvironmentID: "env-trigupd",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with initial trigger value
			{
				Config: testDeploymentConfigWithTriggers(mockServer.URL, "env-trigupd", "proj-trigupd", map[string]string{
					"compose": "hash-v1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "hash-v1"),
					resource.TestCheckResourceAttrSet("arcane_project_deployment.test", "last_deployed_at"),
				),
			},
			// Step 2: Change trigger value -- actual apply to verify update works
			{
				Config: testDeploymentConfigWithTriggers(mockServer.URL, "env-trigupd", "proj-trigupd", map[string]string{
					"compose": "hash-v2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "hash-v2"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
					resource.TestCheckResourceAttrSet("arcane_project_deployment.test", "last_deployed_at"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenPullDisabled_WhenPullEnabled_ThenRedeployed
// validates that changing pull from false to true triggers a redeploy.
func TestProjectDeploymentResource_GivenPullDisabled_WhenPullEnabled_ThenRedeployed(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-pullupd"] = &client.Environment{
		ID:   "env-pullupd",
		Name: "pullupd-env",
	}
	mockServer.HealthyEnvs["env-pullupd"] = true
	mockServer.AddProject("env-pullupd", &client.Project{
		ID:            "proj-pullupd",
		Name:          "pullupd-project",
		Status:        "stopped",
		EnvironmentID: "env-pullupd",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with pull=false
			{
				Config: testDeploymentConfigAllOptions(mockServer.URL, "env-pullupd", "proj-pullupd", false, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "pull", "false"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
			// Step 2: Update to pull=true -- actual apply
			{
				Config: testDeploymentConfigAllOptions(mockServer.URL, "env-pullupd", "proj-pullupd", true, false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "pull", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
					resource.TestCheckResourceAttrSet("arcane_project_deployment.test", "last_deployed_at"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenDefaultStopOnDelete_WhenDestroyed_ThenRemovedFromState
// validates that destroying the deployment with default stop_on_delete (false) removes from state.
func TestProjectDeploymentResource_GivenDefaultStopOnDelete_WhenDestroyed_ThenRemovedFromState(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-destroy"] = &client.Environment{
		ID:   "env-destroy",
		Name: "destroy-env",
	}
	mockServer.HealthyEnvs["env-destroy"] = true
	mockServer.AddProject("env-destroy", &client.Project{
		ID:            "proj-destroy",
		Name:          "destroy-project",
		Status:        "stopped",
		EnvironmentID: "env-destroy",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the deployment
			{
				Config: testDeploymentConfig(mockServer.URL, "env-destroy", "proj-destroy"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
			// Step 2: Destroy by removing from config
			{
				Config: testDeploymentConfigEmpty(mockServer.URL),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenCompositeID_WhenImported_ThenStatePopulated
// validates that importing by environment_id/project_id populates the state correctly.
func TestProjectDeploymentResource_GivenCompositeID_WhenImported_ThenStatePopulated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-import"] = &client.Environment{
		ID:   "env-import",
		Name: "import-env",
	}
	mockServer.HealthyEnvs["env-import"] = true
	mockServer.AddProject("env-import", &client.Project{
		ID:            "proj-import",
		Name:          "import-project",
		Status:        "running",
		EnvironmentID: "env-import",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the deployment
			{
				Config: testDeploymentConfig(mockServer.URL, "env-import", "proj-import"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "id", "env-import/proj-import"),
				),
			},
			// Step 2: Import by composite ID
			{
				ResourceName:      "arcane_project_deployment.test",
				ImportState:       true,
				ImportStateId:     "env-import/proj-import",
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"last_deployed_at",
					"triggers",
					"wait_timeout",
					"pull",
					"force_recreate",
					"remove_orphans",
					"stop_on_delete",
				},
			},
		},
	})
}

// TestProjectDeploymentResource_GivenCustomWaitTimeout_WhenCreated_ThenTimeoutSet
// validates that a custom wait_timeout is stored in state.
func TestProjectDeploymentResource_GivenCustomWaitTimeout_WhenCreated_ThenTimeoutSet(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-timeout"] = &client.Environment{
		ID:   "env-timeout",
		Name: "timeout-env",
	}
	mockServer.HealthyEnvs["env-timeout"] = true
	mockServer.AddProject("env-timeout", &client.Project{
		ID:            "proj-timeout",
		Name:          "timeout-project",
		Status:        "stopped",
		EnvironmentID: "env-timeout",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDeploymentConfigWithTimeout(mockServer.URL, "env-timeout", "proj-timeout", "30s"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "wait_timeout", "30s"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenEnvironmentResource_WhenDeploymentCreated_ThenUsesEnvironmentID
// validates that the deployment resource can reference an arcane_environment resource.
func TestProjectDeploymentResource_GivenEnvironmentResource_WhenDeploymentCreated_ThenUsesEnvironmentID(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate project for the expected environment ID.
	// Mock server generates ID as "env-{name}" when creating via resource.
	mockServer.AddProject("env-dep-env", &client.Project{
		ID:            "proj-dep",
		Name:          "dep-project",
		Status:        "stopped",
		EnvironmentID: "env-dep-env",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testDeploymentConfigWithEnvResource(mockServer.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"arcane_project_deployment.test", "environment_id",
						"arcane_environment.test", "id",
					),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
		},
	})
}

// --- Config helpers ---

func testDeploymentConfig(url, envID, projectID string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_project_deployment" "test" {
  environment_id = %[2]q
  project_id     = %[3]q
}
`, url, envID, projectID)
}

func testDeploymentConfigAllOptions(url, envID, projectID string, pull, forceRecreate, removeOrphans bool) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_project_deployment" "test" {
  environment_id = %[2]q
  project_id     = %[3]q
  pull           = %[4]t
  force_recreate = %[5]t
  remove_orphans = %[6]t
}
`, url, envID, projectID, pull, forceRecreate, removeOrphans)
}

func testDeploymentConfigWithTriggers(url, envID, projectID string, triggers map[string]string) string {
	triggerLines := ""
	for k, v := range triggers {
		triggerLines += fmt.Sprintf("    %s = %q\n", k, v)
	}
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_project_deployment" "test" {
  environment_id = %[2]q
  project_id     = %[3]q
  triggers = {
%[4]s  }
}
`, url, envID, projectID, triggerLines)
}

func testDeploymentConfigWithTimeout(url, envID, projectID, timeout string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_project_deployment" "test" {
  environment_id = %[2]q
  project_id     = %[3]q
  wait_timeout   = %[4]q
}
`, url, envID, projectID, timeout)
}

func testDeploymentConfigEmpty(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}
`, url)
}

func testDeploymentConfigWithEnvResource(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = "dep-env"
  api_url = "http://10.100.1.100:3553"
}

resource "arcane_project_deployment" "test" {
  environment_id = arcane_environment.test.id
  project_id     = "proj-dep"
}
`, url)
}

func testDeploymentConfigWithStopOnDelete(url, envID, projectID string, stopOnDelete bool) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_project_deployment" "test" {
  environment_id = %[2]q
  project_id     = %[3]q
  stop_on_delete = %[4]t
}
`, url, envID, projectID, stopOnDelete)
}

// --- Edge case lifecycle tests ---

// TestProjectDeploymentResource_GivenStopOnDeleteTrue_WhenDestroyed_ThenProjectStopped
// validates that destroying with stop_on_delete=true calls the down endpoint.
func TestProjectDeploymentResource_GivenStopOnDeleteTrue_WhenDestroyed_ThenProjectStopped(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-stopd"] = &client.Environment{
		ID:   "env-stopd",
		Name: "stopd-env",
	}
	mockServer.HealthyEnvs["env-stopd"] = true
	mockServer.AddProject("env-stopd", &client.Project{
		ID:            "proj-stopd",
		Name:          "stopd-project",
		Status:        "stopped",
		EnvironmentID: "env-stopd",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with stop_on_delete=true
			{
				Config: testDeploymentConfigWithStopOnDelete(mockServer.URL, "env-stopd", "proj-stopd", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "stop_on_delete", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
			// Step 2: Destroy -- should call /down and project should be stopped
			{
				Config: testDeploymentConfigEmpty(mockServer.URL),
			},
		},
	})

	// After destroy, verify the mock project was stopped
	project := mockServer.Projects["env-stopd"]["proj-stopd"]
	if project.Status != "stopped" {
		t.Errorf("expected project status 'stopped' after stop_on_delete destroy, got %q", project.Status)
	}
}

// TestProjectDeploymentResource_GivenTriggersUnchanged_WhenPlanned_ThenNoDiff
// validates that re-applying the same triggers produces a clean plan (no diff).
func TestProjectDeploymentResource_GivenTriggersUnchanged_WhenPlanned_ThenNoDiff(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-nodiff"] = &client.Environment{
		ID:   "env-nodiff",
		Name: "nodiff-env",
	}
	mockServer.HealthyEnvs["env-nodiff"] = true
	mockServer.AddProject("env-nodiff", &client.Project{
		ID:            "proj-nodiff",
		Name:          "nodiff-project",
		Status:        "stopped",
		EnvironmentID: "env-nodiff",
	})

	config := testDeploymentConfigWithTriggers(mockServer.URL, "env-nodiff", "proj-nodiff", map[string]string{
		"compose": "same-hash",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "same-hash"),
				),
			},
			// Step 2: Re-apply identical config -- should produce empty plan
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// TestProjectDeploymentResource_GivenProjectStoppedExternally_WhenRead_ThenStatusReflected
// validates that if a project is stopped outside Terraform, Read picks up
// the new status in state. Since status is computed-only, Terraform won't
// re-deploy automatically — the user would use triggers or -replace.
func TestProjectDeploymentResource_GivenProjectStoppedExternally_WhenRead_ThenStatusReflected(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-drift"] = &client.Environment{
		ID:   "env-drift",
		Name: "drift-env",
	}
	mockServer.HealthyEnvs["env-drift"] = true
	mockServer.AddProject("env-drift", &client.Project{
		ID:            "proj-drift",
		Name:          "drift-project",
		Status:        "stopped",
		EnvironmentID: "env-drift",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the deployment
			{
				Config: testDeploymentConfig(mockServer.URL, "env-drift", "proj-drift"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
				),
			},
			// Step 2: Simulate external drift — project was stopped outside TF.
			// Read reflects the new status in state.
			{
				PreConfig: func() {
					mockServer.Projects["env-drift"]["proj-drift"].Status = "stopped"
				},
				Config: testDeploymentConfig(mockServer.URL, "env-drift", "proj-drift"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "stopped"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenMultipleTriggersChanged_WhenUpdated_ThenAllUpdated
// validates that changing multiple triggers at once triggers update with all new values.
func TestProjectDeploymentResource_GivenMultipleTriggersChanged_WhenUpdated_ThenAllUpdated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-multi"] = &client.Environment{
		ID:   "env-multi",
		Name: "multi-env",
	}
	mockServer.HealthyEnvs["env-multi"] = true
	mockServer.AddProject("env-multi", &client.Project{
		ID:            "proj-multi",
		Name:          "multi-project",
		Status:        "stopped",
		EnvironmentID: "env-multi",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with two triggers
			{
				Config: testDeploymentConfigWithTriggers(mockServer.URL, "env-multi", "proj-multi", map[string]string{
					"compose": "hash-v1",
					"env":     "env-v1",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "hash-v1"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.env", "env-v1"),
				),
			},
			// Step 2: Change both triggers
			{
				Config: testDeploymentConfigWithTriggers(mockServer.URL, "env-multi", "proj-multi", map[string]string{
					"compose": "hash-v2",
					"env":     "env-v2",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.compose", "hash-v2"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "triggers.env", "env-v2"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "status", "running"),
					resource.TestCheckResourceAttrSet("arcane_project_deployment.test", "last_deployed_at"),
				),
			},
		},
	})
}

// TestProjectDeploymentResource_GivenOptionsUnchanged_WhenPlanned_ThenNoDiff
// validates that re-applying the same options produces a clean plan.
func TestProjectDeploymentResource_GivenOptionsUnchanged_WhenPlanned_ThenNoDiff(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	mockServer.Environments["env-optnodiff"] = &client.Environment{
		ID:   "env-optnodiff",
		Name: "optnodiff-env",
	}
	mockServer.HealthyEnvs["env-optnodiff"] = true
	mockServer.AddProject("env-optnodiff", &client.Project{
		ID:            "proj-optnodiff",
		Name:          "optnodiff-project",
		Status:        "stopped",
		EnvironmentID: "env-optnodiff",
	})

	config := testDeploymentConfigAllOptions(mockServer.URL, "env-optnodiff", "proj-optnodiff", true, false, true)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "pull", "true"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "force_recreate", "false"),
					resource.TestCheckResourceAttr("arcane_project_deployment.test", "remove_orphans", "true"),
				),
			},
			// Step 2: Re-apply identical config -- should produce empty plan
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
