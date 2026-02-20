package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// TestProjectDataSource_GivenExistingProject_WhenLookedUpByID_ThenReturnsProject
// validates that a project can be looked up by ID within an environment.
func TestProjectDataSource_GivenExistingProject_WhenLookedUpByID_ThenReturnsProject(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate mock server
	mockServer.Environments["env-1"] = &client.Environment{
		ID:   "env-1",
		Name: "test-env",
	}
	mockServer.AddProject("env-1", &client.Project{
		ID:            "proj-webapp",
		Name:          "webapp",
		Status:        "running",
		Path:          "/opt/stacks/webapp",
		EnvironmentID: "env-1",
		Services: []client.ProjectService{
			{Name: "web", Status: "running", Image: "nginx:latest"},
			{Name: "api", Status: "running", Image: "myapp:v1"},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjectDataSourceConfigByID(mockServer.URL, "env-1", "proj-webapp"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_project.test", "id", "proj-webapp"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "name", "webapp"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "status", "running"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "path", "/opt/stacks/webapp"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "services.#", "2"),
				),
			},
		},
	})
}

// TestProjectDataSource_GivenExistingProject_WhenLookedUpByName_ThenReturnsProject
// validates that a project can be looked up by name within an environment.
func TestProjectDataSource_GivenExistingProject_WhenLookedUpByName_ThenReturnsProject(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate mock server
	mockServer.Environments["env-2"] = &client.Environment{
		ID:   "env-2",
		Name: "production",
	}
	mockServer.AddProject("env-2", &client.Project{
		ID:            "proj-api",
		Name:          "api-service",
		Status:        "stopped",
		EnvironmentID: "env-2",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjectDataSourceConfigByName(mockServer.URL, "env-2", "api-service"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_project.test", "id", "proj-api"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "name", "api-service"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "status", "stopped"),
				),
			},
		},
	})
}

// TestProjectDataSource_GivenProjectWithServices_WhenRead_ThenServicesPopulated
// validates that project services are properly populated.
func TestProjectDataSource_GivenProjectWithServices_WhenRead_ThenServicesPopulated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate mock server
	mockServer.Environments["env-3"] = &client.Environment{
		ID:   "env-3",
		Name: "staging",
	}
	mockServer.AddProject("env-3", &client.Project{
		ID:            "proj-stack",
		Name:          "full-stack",
		Status:        "running",
		EnvironmentID: "env-3",
		Services: []client.ProjectService{
			{Name: "frontend", Status: "running", Image: "frontend:latest"},
			{Name: "backend", Status: "running", Image: "backend:v2"},
			{Name: "database", Status: "running", Image: "postgres:15"},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjectDataSourceConfigByID(mockServer.URL, "env-3", "proj-stack"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_project.test", "services.#", "3"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "services.0.name", "frontend"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "services.0.status", "running"),
					resource.TestCheckResourceAttr("data.arcane_project.test", "services.0.image", "frontend:latest"),
				),
			},
		},
	})
}

// TestProjectDataSource_GivenEnvironmentCreatedByResource_WhenProjectLookedUp_ThenSucceeds
// validates project lookup using environment ID from a resource.
func TestProjectDataSource_GivenEnvironmentCreatedByResource_WhenProjectLookedUp_ThenSucceeds(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// The mock server will create the environment when the resource is applied,
	// so we need to pre-populate the project for the expected environment ID
	// Note: mock server generates ID as "env-{name}"
	mockServer.AddProject("env-dynamic-env", &client.Project{
		ID:            "proj-dynamic",
		Name:          "dynamic-project",
		Status:        "running",
		EnvironmentID: "env-dynamic-env",
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testProjectDataSourceConfigWithEnvironmentResource(mockServer.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_project.test", "name", "dynamic-project"),
				),
			},
		},
	})
}

func testProjectDataSourceConfigByID(url, envID, projectID string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

data "arcane_project" "test" {
  environment_id = %[2]q
  id             = %[3]q
}
`, url, envID, projectID)
}

func testProjectDataSourceConfigByName(url, envID, projectName string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

data "arcane_project" "test" {
  environment_id = %[2]q
  name           = %[3]q
}
`, url, envID, projectName)
}

func testProjectDataSourceConfigWithEnvironmentResource(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = "dynamic-env"
  api_url = "http://10.100.1.100:3553"
}

data "arcane_project" "test" {
  environment_id = arcane_environment.test.id
  name           = "dynamic-project"
}
`, url)
}
