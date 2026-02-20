package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestGitRepositoryResource_GivenValidConfig_WhenCreated_ThenRepositoryExists
// validates that a git repository resource can be created with name and url,
// and that the id is set and branch is computed to the default ("main").
func TestGitRepositoryResource_GivenValidConfig_WhenCreated_ThenRepositoryExists(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGitRepositoryResourceConfig(mockServer.URL, "test-repo", "https://github.com/example/repo.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_git_repository.test", "id"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "test-repo"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "url", "https://github.com/example/repo.git"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "branch", "main"),
				),
			},
		},
	})
}

// TestGitRepositoryResource_GivenBranchAndAuth_WhenCreated_ThenAllFieldsSet
// validates that a git repository can be created with all optional fields
// (branch, auth_type, credentials) and that they are correctly stored.
func TestGitRepositoryResource_GivenBranchAndAuth_WhenCreated_ThenAllFieldsSet(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGitRepositoryResourceConfigFull(mockServer.URL, "auth-repo", "https://github.com/example/private.git", "develop", "token", "secret-token"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_git_repository.test", "id"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "auth-repo"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "url", "https://github.com/example/private.git"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "branch", "develop"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "auth_type", "token"),
				),
			},
		},
	})
}

// TestGitRepositoryResource_GivenExistingRepo_WhenNameUpdated_ThenChangesApplied
// validates that updating the name on an existing git repository applies correctly.
func TestGitRepositoryResource_GivenExistingRepo_WhenNameUpdated_ThenChangesApplied(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create initial repository
			{
				Config: testGitRepositoryResourceConfig(mockServer.URL, "original-name", "https://github.com/example/repo.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "original-name"),
				),
			},
			// Update the name
			{
				Config: testGitRepositoryResourceConfig(mockServer.URL, "updated-name", "https://github.com/example/repo.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "updated-name"),
				),
			},
		},
	})
}

// TestGitRepositoryResource_GivenExistingRepo_WhenDeleted_ThenRemoved
// validates that a git repository can be created and then destroyed cleanly.
func TestGitRepositoryResource_GivenExistingRepo_WhenDeleted_ThenRemoved(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testGitRepositoryResourceConfig(mockServer.URL, "delete-repo", "https://github.com/example/repo.git"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("arcane_git_repository.test", "id"),
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "delete-repo"),
				),
			},
		},
	})
}

// TestGitRepositoryResource_GivenExistingRepo_WhenImported_ThenStateMatches
// validates that a git repository can be imported by ID and that state is verified.
// Credentials are excluded from import verification since the API does not return them.
func TestGitRepositoryResource_GivenExistingRepo_WhenImported_ThenStateMatches(t *testing.T) {
	mockServer := NewMockServer()
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create the repository first
			{
				Config: testGitRepositoryResourceConfigFull(mockServer.URL, "import-repo", "https://github.com/example/repo.git", "main", "token", "my-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("arcane_git_repository.test", "name", "import-repo"),
					resource.TestCheckResourceAttrSet("arcane_git_repository.test", "id"),
				),
			},
			// Import the repository by ID
			{
				ResourceName:            "arcane_git_repository.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credentials"},
			},
		},
	})
}

// --- Config helpers ---

func testGitRepositoryResourceConfig(url, name, repoURL string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_git_repository" "test" {
  name = %[2]q
  url  = %[3]q
}
`, url, name, repoURL)
}

func testGitRepositoryResourceConfigFull(url, name, repoURL, branch, authType, credentials string) string {
	return fmt.Sprintf(`
provider "arcane" {
  url = %[1]q
}

resource "arcane_git_repository" "test" {
  name        = %[2]q
  url         = %[3]q
  branch      = %[4]q
  auth_type   = %[5]q
  credentials = %[6]q
}
`, url, name, repoURL, branch, authType, credentials)
}
