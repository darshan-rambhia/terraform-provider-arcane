data "arcane_project_status" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id
}

output "container_health" {
  value = data.arcane_project_status.webapp.containers
}
