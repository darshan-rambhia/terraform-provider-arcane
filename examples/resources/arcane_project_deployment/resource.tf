# Look up the environment
data "arcane_environment" "production" {
  name = "production"
}

# Look up the project to deploy
data "arcane_project" "webapp" {
  environment_id = data.arcane_environment.production.id
  name           = "webapp"
}

# Deploy the project with default options
resource "arcane_project_deployment" "webapp" {
  environment_id = data.arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id
}

# Deploy another project with all options
resource "arcane_project_deployment" "api" {
  environment_id = data.arcane_environment.production.id
  project_id     = data.arcane_project.api.id

  # Pull latest images before deploying
  pull = true

  # Force recreate containers even if config hasn't changed
  force_recreate = true

  # Remove containers for services not in the compose file
  remove_orphans = true
}

# Output the deployment status
output "webapp_status" {
  value = arcane_project_deployment.webapp.status
}
