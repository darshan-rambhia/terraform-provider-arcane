# Look up an existing environment
data "arcane_environment" "production" {
  name = "production"
}

# Look up a project by name within the environment
data "arcane_project" "webapp" {
  environment_id = data.arcane_environment.production.id
  name           = "webapp"
}

# Look up a project by ID
data "arcane_project" "by_id" {
  environment_id = data.arcane_environment.production.id
  id             = "project-xyz789"
}

# Use the project data
output "project_status" {
  value = data.arcane_project.webapp.status
}

output "project_path" {
  value = data.arcane_project.webapp.path
}
