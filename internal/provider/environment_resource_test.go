package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestEnvironmentResource_GivenValidConfig_WhenCreated_ThenEnvironmentExists
// validates that an environment resource can be created with name, api_url, and description.
func TestEnvironmentResource_GivenValidConfig_WhenCreated_ThenEnvironmentExists(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentResourceConfig(mockServer.URL, "test-env", "http://10.100.1.100:3553", "Test environment", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "name", "test-env"),
					resource.TestCheckResourceAttr("arcane_environment.test", "api_url", "http://10.100.1.100:3553"),
					resource.TestCheckResourceAttr("arcane_environment.test", "description", "Test environment"),
					resource.TestCheckResourceAttr("arcane_environment.test", "use_api_key", "false"),
					resource.TestCheckResourceAttrSet("arcane_environment.test", "id"),
				),
			},
		},
	})
}

// TestEnvironmentResource_GivenUseAPIKeyEnabled_WhenCreated_ThenAccessTokenGenerated
// validates that when use_api_key is true, an access token is generated on create.
func TestEnvironmentResource_GivenUseAPIKeyEnabled_WhenCreated_ThenAccessTokenGenerated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentResourceConfig(mockServer.URL, "secure-env", "http://10.100.1.101:3553", "Secure environment", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "name", "secure-env"),
					resource.TestCheckResourceAttr("arcane_environment.test", "use_api_key", "true"),
					resource.TestCheckResourceAttrSet("arcane_environment.test", "access_token"),
				),
			},
		},
	})
}

// TestEnvironmentResource_GivenExistingEnvironment_WhenDescriptionUpdated_ThenChangesApplied
// validates that updating the description on an existing environment applies correctly.
func TestEnvironmentResource_GivenExistingEnvironment_WhenDescriptionUpdated_ThenChangesApplied(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create initial environment
			{
				Config: testEnvironmentResourceConfig(mockServer.URL, "update-env", "http://10.100.1.102:3553", "Initial description", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "description", "Initial description"),
				),
			},
			// Update the description
			{
				Config: testEnvironmentResourceConfig(mockServer.URL, "update-env", "http://10.100.1.102:3553", "Updated description", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "description", "Updated description"),
				),
			},
		},
	})
}

// TestEnvironmentResource_GivenMinimalConfig_WhenCreated_ThenDefaultsApplied
// validates that an environment can be created with only the required fields (name + api_url).
func TestEnvironmentResource_GivenMinimalConfig_WhenCreated_ThenDefaultsApplied(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentResourceConfigMinimal(mockServer.URL, "minimal-env", "http://10.100.1.103:3553"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "name", "minimal-env"),
					resource.TestCheckResourceAttr("arcane_environment.test", "api_url", "http://10.100.1.103:3553"),
					resource.TestCheckResourceAttr("arcane_environment.test", "use_api_key", "false"),
					resource.TestCheckResourceAttrSet("arcane_environment.test", "id"),
				),
			},
		},
	})
}

// TestEnvironmentResource_GivenExistingEnvironment_WhenImported_ThenStateMatches
// validates that an environment can be imported by ID and state is verified.
func TestEnvironmentResource_GivenExistingEnvironment_WhenImported_ThenStateMatches(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create the environment first
			{
				Config: testEnvironmentResourceConfig(mockServer.URL, "import-env", "http://10.100.1.104:3553", "Environment for import test", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_environment.test", "name", "import-env"),
					resource.TestCheckResourceAttrSet("arcane_environment.test", "id"),
				),
			},
			// Import the environment by ID
			{
				ResourceName:            "arcane_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"access_token", "regenerate_access_token", "api_url"},
			},
		},
	})
}

func testEnvironmentResourceConfig(url, name, apiURL, description string, useAPIKey bool) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name        = %[2]q
  api_url     = %[3]q
  description = %[4]q
  use_api_key = %[5]t
}
`, url, name, apiURL, description, useAPIKey)
}

func testEnvironmentResourceConfigMinimal(url, name, apiURL string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = %[3]q
}
`, url, name, apiURL)
}
