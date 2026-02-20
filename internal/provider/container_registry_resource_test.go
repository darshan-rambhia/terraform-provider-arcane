package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestContainerRegistryResource_GivenValidConfig_WhenCreated_ThenRegistryExists
// validates that a container registry resource can be created with name and url.
func TestContainerRegistryResource_GivenValidConfig_WhenCreated_ThenRegistryExists(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerRegistryResourceConfig(mockServer.URL, "test-registry", "https://ghcr.io"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_container_registry.test", "id"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "test-registry"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "url", "https://ghcr.io"),
				),
			},
		},
	})
}

// TestContainerRegistryResource_GivenAuthConfig_WhenCreated_ThenAuthFieldsSet
// validates that a container registry created with auth fields has auth_type and username set.
func TestContainerRegistryResource_GivenAuthConfig_WhenCreated_ThenAuthFieldsSet(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerRegistryResourceConfigFull(mockServer.URL, "auth-registry", "https://ghcr.io", "basic", "my-user", "my-secret-token"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_container_registry.test", "id"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "auth-registry"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "url", "https://ghcr.io"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "auth_type", "basic"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "username", "my-user"),
				),
			},
		},
	})
}

// TestContainerRegistryResource_GivenExistingRegistry_WhenNameUpdated_ThenChangesApplied
// validates that updating the name on an existing container registry applies correctly.
func TestContainerRegistryResource_GivenExistingRegistry_WhenNameUpdated_ThenChangesApplied(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create initial registry
			{
				Config: testContainerRegistryResourceConfig(mockServer.URL, "original-name", "https://ghcr.io"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "original-name"),
				),
			},
			// Update the name
			{
				Config: testContainerRegistryResourceConfig(mockServer.URL, "updated-name", "https://ghcr.io"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "updated-name"),
				),
			},
		},
	})
}

// TestContainerRegistryResource_GivenExistingRegistry_WhenDeleted_ThenRemoved
// validates that a container registry is removed on destroy (happens naturally with resource.Test).
func TestContainerRegistryResource_GivenExistingRegistry_WhenDeleted_ThenRemoved(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testContainerRegistryResourceConfig(mockServer.URL, "delete-registry", "https://index.docker.io/v1/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_container_registry.test", "id"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "delete-registry"),
					resource.TestCheckResourceAttr("arcane_container_registry.test", "url", "https://index.docker.io/v1/"),
				),
			},
		},
	})
}

// TestContainerRegistryResource_GivenExistingRegistry_WhenImported_ThenStateMatches
// validates that a container registry can be imported by ID and state is verified.
func TestContainerRegistryResource_GivenExistingRegistry_WhenImported_ThenStateMatches(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create the registry first
			{
				Config: testContainerRegistryResourceConfigFull(mockServer.URL, "import-registry", "https://ghcr.io", "basic", "import-user", "import-pass"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_container_registry.test", "name", "import-registry"),
					resource.TestCheckResourceAttrSet("arcane_container_registry.test", "id"),
				),
			},
			// Import the registry by ID
			{
				ResourceName:            "arcane_container_registry.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

// --- Config helpers ---

func testContainerRegistryResourceConfig(url, name, regURL string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_container_registry" "test" {
  name = %[2]q
  url  = %[3]q
}
`, url, name, regURL)
}

func testContainerRegistryResourceConfigFull(url, name, regURL, authType, username, password string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_container_registry" "test" {
  name      = %[2]q
  url       = %[3]q
  auth_type = %[4]q
  username  = %[5]q
  password  = %[6]q
}
`, url, name, regURL, authType, username, password)
}
