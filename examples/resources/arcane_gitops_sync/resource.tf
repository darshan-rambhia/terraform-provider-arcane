resource "arcane_gitops_sync" "webapp" {
  environment_id = arcane_environment.production.id
  repository_id  = arcane_git_repository.infra.id
  path           = "apps/webapp"
  branch         = "main"
  compose_file   = "docker-compose.yml"
  auto_sync      = true
  sync_interval  = "5m"
}
