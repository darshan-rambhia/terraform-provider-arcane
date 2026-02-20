package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// TestEnvironmentDataSource_GivenExistingEnvironment_WhenLookedUpByID_ThenReturnsEnvironment
// validates that an environment can be looked up by ID.
func TestEnvironmentDataSource_GivenExistingEnvironment_WhenLookedUpByID_ThenReturnsEnvironment(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate mock server with an environment
	mockServer.Environments["env-test"] = &client.Environment{
		ID:          "env-test",
		Name:        "test-environment",
		Description: "A test environment",
		UseAPIKey:   false,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentDataSourceConfigByID(mockServer.URL, "env-test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_environment.test", "id", "env-test"),
					resource.TestCheckResourceAttr("data.arcane_environment.test", "name", "test-environment"),
					resource.TestCheckResourceAttr("data.arcane_environment.test", "description", "A test environment"),
					resource.TestCheckResourceAttr("data.arcane_environment.test", "use_api_key", "false"),
				),
			},
		},
	})
}

// TestEnvironmentDataSource_GivenExistingEnvironment_WhenLookedUpByName_ThenReturnsEnvironment
// validates that an environment can be looked up by name.
func TestEnvironmentDataSource_GivenExistingEnvironment_WhenLookedUpByName_ThenReturnsEnvironment(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	// Pre-populate mock server with an environment
	mockServer.Environments["env-named"] = &client.Environment{
		ID:          "env-named",
		Name:        "named-environment",
		Description: "Environment looked up by name",
		UseAPIKey:   true,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentDataSourceConfigByName(mockServer.URL, "named-environment"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_environment.test", "id", "env-named"),
					resource.TestCheckResourceAttr("data.arcane_environment.test", "name", "named-environment"),
					resource.TestCheckResourceAttr("data.arcane_environment.test", "use_api_key", "true"),
				),
			},
		},
	})
}

// TestEnvironmentDataSource_GivenResourceDependency_WhenLookedUp_ThenReturnsCreatedEnvironment
// validates that the data source can read an environment created by a resource.
func TestEnvironmentDataSource_GivenResourceDependency_WhenLookedUp_ThenReturnsCreatedEnvironment(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentDataSourceConfigWithResource(mockServer.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.arcane_environment.test", "id",
						"arcane_environment.source", "id",
					),
					resource.TestCheckResourceAttrPair(
						"data.arcane_environment.test", "name",
						"arcane_environment.source", "name",
					),
				),
			},
		},
	})
}

func testEnvironmentDataSourceConfigByID(url, id string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

data "arcane_environment" "test" {
  id = %[2]q
}
`, url, id)
}

func testEnvironmentDataSourceConfigByName(url, name string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

data "arcane_environment" "test" {
  name = %[2]q
}
`, url, name)
}

func testEnvironmentDataSourceConfigWithResource(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "source" {
  name        = "source-env"
  api_url     = "http://10.100.1.100:3553"
  description = "Source environment"
}

data "arcane_environment" "test" {
  id = arcane_environment.source.id
}
`, url)
}
