package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestGitOpsSyncResource_GivenValidConfig_WhenCreated_ThenSyncExists
// validates that a gitops sync can be created with minimal config (environment + git repo),
// and that defaults for compose_file and auto_sync are applied.
func TestGitOpsSyncResource_GivenValidConfig_WhenCreated_ThenSyncExists(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGitOpsSyncResourceConfig(mockServer.URL, "sync-env", "infra-repo", "https://github.com/example/infra.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "id"),
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "environment_id"),
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "repository_id"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "compose_file", "docker-compose.yml"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "auto_sync", "false"),
				),
			},
		},
	})
}

// TestGitOpsSyncResource_GivenFullConfig_WhenCreated_ThenAllFieldsSet
// validates that all optional fields (path, branch, compose_file, sync_interval, auto_sync)
// are correctly stored in state when provided.
func TestGitOpsSyncResource_GivenFullConfig_WhenCreated_ThenAllFieldsSet(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGitOpsSyncResourceConfigFull(
					mockServer.URL,
					"full-env",
					"full-repo",
					"https://github.com/example/full.git",
					"apps/webapp",
					"develop",
					"compose.prod.yml",
					"5m",
					true,
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "id"),
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "environment_id"),
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "repository_id"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "path", "apps/webapp"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "branch", "develop"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "compose_file", "compose.prod.yml"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "sync_interval", "5m"),
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "auto_sync", "true"),
				),
			},
		},
	})
}

// TestGitOpsSyncResource_GivenExistingSync_WhenAutoSyncUpdated_ThenChangesApplied
// validates that updating auto_sync from false to true is applied correctly.
func TestGitOpsSyncResource_GivenExistingSync_WhenAutoSyncUpdated_ThenChangesApplied(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create with auto_sync=false
			{
				Config: testGitOpsSyncResourceConfigWithAutoSync(mockServer.URL, "update-env", "update-repo", "https://github.com/example/update.git", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "auto_sync", "false"),
				),
			},
			// Step 2: Update to auto_sync=true
			{
				Config: testGitOpsSyncResourceConfigWithAutoSync(mockServer.URL, "update-env", "update-repo", "https://github.com/example/update.git", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_gitops_sync.test", "auto_sync", "true"),
				),
			},
		},
	})
}

// TestGitOpsSyncResource_GivenExistingSync_WhenDeleted_ThenRemoved
// validates that destroying a gitops sync resource removes it from state.
func TestGitOpsSyncResource_GivenExistingSync_WhenDeleted_ThenRemoved(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the gitops sync
			{
				Config: testGitOpsSyncResourceConfig(mockServer.URL, "delete-env", "delete-repo", "https://github.com/example/delete.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "id"),
				),
			},
			// Step 2: Destroy by removing gitops sync from config
			{
				Config: testGitOpsSyncResourceConfigEmpty(mockServer.URL),
			},
		},
	})
}

// TestGitOpsSyncResource_GivenCompositeID_WhenImported_ThenStatePopulated
// validates that importing by environment_id/sync_id populates the state correctly.
func TestGitOpsSyncResource_GivenCompositeID_WhenImported_ThenStatePopulated(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: Create the gitops sync
			{
				Config: testGitOpsSyncResourceConfig(mockServer.URL, "import-env", "import-repo", "https://github.com/example/import.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "id"),
					resource.TestCheckResourceAttrSet("arcane_gitops_sync.test", "environment_id"),
				),
			},
			// Step 2: Import by composite ID (environment_id/sync_id)
			{
				ResourceName: "arcane_gitops_sync.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["arcane_gitops_sync.test"]
					return rs.Primary.Attributes["environment_id"] + "/" + rs.Primary.Attributes["id"], nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"last_sync_at", "last_sync_commit"},
			},
		},
	})
}

// --- Config helpers ---

func testGitOpsSyncResourceConfig(url, envName, repoName, repoURL string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = "http://10.100.1.100:3553"
}

resource "arcane_git_repository" "test" {
  name = %[3]q
  url  = %[4]q
}

resource "arcane_gitops_sync" "test" {
  environment_id = arcane_environment.test.id
  repository_id  = arcane_git_repository.test.id
}
`, url, envName, repoName, repoURL)
}

func testGitOpsSyncResourceConfigFull(url, envName, repoName, repoURL, path, branch, composeFile, syncInterval string, autoSync bool) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = "http://10.100.1.100:3553"
}

resource "arcane_git_repository" "test" {
  name = %[3]q
  url  = %[4]q
}

resource "arcane_gitops_sync" "test" {
  environment_id = arcane_environment.test.id
  repository_id  = arcane_git_repository.test.id
  path           = %[5]q
  branch         = %[6]q
  compose_file   = %[7]q
  sync_interval  = %[8]q
  auto_sync      = %[9]t
}
`, url, envName, repoName, repoURL, path, branch, composeFile, syncInterval, autoSync)
}

func testGitOpsSyncResourceConfigWithAutoSync(url, envName, repoName, repoURL string, autoSync bool) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_environment" "test" {
  name    = %[2]q
  api_url = "http://10.100.1.100:3553"
}

resource "arcane_git_repository" "test" {
  name = %[3]q
  url  = %[4]q
}

resource "arcane_gitops_sync" "test" {
  environment_id = arcane_environment.test.id
  repository_id  = arcane_git_repository.test.id
  auto_sync      = %[5]t
}
`, url, envName, repoName, repoURL, autoSync)
}

func testGitOpsSyncResourceConfigEmpty(url string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}
`, url)
}
