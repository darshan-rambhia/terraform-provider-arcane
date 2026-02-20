package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/darshan-rambhia/terraform-provider-arcane/internal/client"
)

// TestEnvironmentHealthDataSource_GivenHealthyEnvironment_WhenRead_ThenIsConnectedTrue
// validates that a healthy environment returns is_connected=true and empty error_message.
func TestEnvironmentHealthDataSource_GivenHealthyEnvironment_WhenRead_ThenIsConnectedTrue(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	envID := "env-health-1"

	// Pre-populate mock server with a healthy environment
	mockServer.Environments[envID] = &client.Environment{
		ID:   envID,
		Name: "healthy-env",
	}
	mockServer.HealthyEnvs[envID] = true

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentHealthDataSourceConfig(mockServer.URL, envID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_environment_health.test", "environment_id", envID),
					resource.TestCheckResourceAttr("data.arcane_environment_health.test", "is_connected", "true"),
					resource.TestCheckResourceAttr("data.arcane_environment_health.test", "error_message", ""),
				),
			},
		},
	})
}

// TestEnvironmentHealthDataSource_GivenUnhealthyEnvironment_WhenRead_ThenIsConnectedFalse
// validates that an unhealthy environment returns is_connected=false and a non-empty error_message.
func TestEnvironmentHealthDataSource_GivenUnhealthyEnvironment_WhenRead_ThenIsConnectedFalse(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	envID := "env-health-2"

	// Pre-populate mock server with an unhealthy environment
	mockServer.Environments[envID] = &client.Environment{
		ID:   envID,
		Name: "unhealthy-env",
	}
	mockServer.HealthyEnvs[envID] = false

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testEnvironmentHealthDataSourceConfig(mockServer.URL, envID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.arcane_environment_health.test", "environment_id", envID),
					resource.TestCheckResourceAttr("data.arcane_environment_health.test", "is_connected", "false"),
					resource.TestCheckResourceAttrSet("data.arcane_environment_health.test", "error_message"),
				),
			},
		},
	})
}

func testEnvironmentHealthDataSourceConfig(url, envID string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

data "arcane_environment_health" "test" {
  environment_id = %[2]q
}
`, url, envID)
}
