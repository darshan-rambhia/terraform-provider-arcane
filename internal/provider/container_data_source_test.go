package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// TestContainerDataSource_GivenContainerExists_WhenLookedUpByID_ThenReturnsContainer
// validates that a container can be looked up by ID within an environment.
func TestContainerDataSource_GivenContainerExists_WhenLookedUpByID_ThenReturnsContainer(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	envName := "container-id-env"
	envID := "env-" + envName
	projectID := "proj-webapp"

	// Pre-populate mock server with project and containers
	// The environment will be created via HCL resource, generating ID "env-{name}"
	mockServer.AddProject(envID, &client.Project{
		ID:            projectID,
		Name:          "webapp",
		Status:        "running",
		EnvironmentID: envID,
	})
	mockServer.AddContainers(envID, projectID, []client.ContainerDetail{
		{
			ID:     "abc123",
			Name:   "nginx-web",
			Image:  "nginx:latest",
			Status: "running",
			Health: "healthy",
		},
		{
			ID:     "def456",
			Name:   "postgres-db",
			Image:  "postgres:15",
			Status: "running",
			Health: "healthy",
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerDataSourceByIDConfig(mockServer.URL, envName, "abc123"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_container.test", "id", "abc123"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "name", "nginx-web"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "image", "nginx:latest"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "status", "running"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "health", "healthy"),
				),
			},
		},
	})
}

// TestContainerDataSource_GivenContainerExists_WhenLookedUpByName_ThenReturnsContainer
// validates that a container can be looked up by name within an environment.
func TestContainerDataSource_GivenContainerExists_WhenLookedUpByName_ThenReturnsContainer(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	envName := "container-name-env"
	envID := "env-" + envName
	projectID := "proj-api"

	// Pre-populate mock server with project and containers
	mockServer.AddProject(envID, &client.Project{
		ID:            projectID,
		Name:          "api-service",
		Status:        "running",
		EnvironmentID: envID,
	})
	mockServer.AddContainers(envID, projectID, []client.ContainerDetail{
		{
			ID:     "cnt-001",
			Name:   "redis-cache",
			Image:  "redis:7-alpine",
			Status: "running",
			Health: "healthy",
		},
		{
			ID:     "cnt-002",
			Name:   "api-server",
			Image:  "myapp:v2",
			Status: "running",
			Health: "",
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerDataSourceByNameConfig(mockServer.URL, envName, "redis-cache"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_container.test", "id", "cnt-001"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "name", "redis-cache"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "image", "redis:7-alpine"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "status", "running"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "health", "healthy"),
				),
			},
		},
	})
}

// TestContainerDataSource_GivenContainerWithPorts_WhenRead_ThenPortsPopulated
// validates that container port mappings are properly populated.
func TestContainerDataSource_GivenContainerWithPorts_WhenRead_ThenPortsPopulated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	envName := "container-ports-env"
	envID := "env-" + envName
	projectID := "proj-ports"

	// Pre-populate mock server with project and a container that has ports
	mockServer.AddProject(envID, &client.Project{
		ID:            projectID,
		Name:          "port-test",
		Status:        "running",
		EnvironmentID: envID,
	})
	mockServer.AddContainers(envID, projectID, []client.ContainerDetail{
		{
			ID:     "port-container-1",
			Name:   "traefik",
			Image:  "traefik:v3",
			Status: "running",
			Health: "healthy",
			Ports: []client.ContainerPort{
				{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
				{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
				{HostPort: 8080, ContainerPort: 8080, Protocol: "tcp"},
			},
		},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerDataSourceByIDConfig(mockServer.URL, envName, "port-container-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_container.test", "id", "port-container-1"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "name", "traefik"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.#", "3"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.0.host_port", "80"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.0.container_port", "80"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.0.protocol", "tcp"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.1.host_port", "443"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.1.container_port", "443"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.1.protocol", "tcp"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.2.host_port", "8080"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.2.container_port", "8080"),
					resource.TestCheckResourceAttr("data.arcane_container.test", "ports.2.protocol", "tcp"),
				),
			},
		},
	})
}

// TestContainerDataSource_GivenNoIDOrName_WhenRead_ThenError
// validates that an error is returned when neither id nor name is specified.
func TestContainerDataSource_GivenNoIDOrName_WhenRead_ThenError(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerDataSourceNoIDOrNameConfig(mockServer.URL),
				ExpectError: regexp.MustCompile(
					`(?i)either.*"id".*or.*"name".*must be specified`,
				),
			},
		},
	})
}

func testContainerDataSourceByIDConfig(url, envName, containerID string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = "http://10.100.1.100:3553"
}

data "arcane_container" "test" {
  environment_id = arcane_environment.test.id
  id             = %[3]q
}
`, url, envName, containerID)
}

func testContainerDataSourceByNameConfig(url, envName, containerName string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = "http://10.100.1.100:3553"
}

data "arcane_container" "test" {
  environment_id = arcane_environment.test.id
  name           = %[3]q
}
`, url, envName, containerName)
}

func testContainerDataSourceNoIDOrNameConfig(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = "no-id-or-name-env"
  api_url = "http://10.100.1.100:3553"
}

data "arcane_container" "test" {
  environment_id = arcane_environment.test.id
}
`, url)
}
