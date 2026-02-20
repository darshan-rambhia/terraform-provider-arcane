resource "arcane_git_repository" "infra" {
  name        = "homelab-infra"
  url         = "https://github.com/example/homelab-infra.git"
  branch      = "main"
  auth_type   = "token"
  credentials = var.github_token
}
