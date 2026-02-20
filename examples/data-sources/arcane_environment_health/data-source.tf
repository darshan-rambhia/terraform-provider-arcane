data "arcane_environment_health" "production" {
  environment_id = arcane_environment.production.id
}

resource "arcane_project_deployment" "webapp" {
  environment_id = arcane_environment.production.id
  project_id     = data.arcane_project.webapp.id

  lifecycle {
    precondition {
      condition     = data.arcane_environment_health.production.is_connected
      error_message = "Agent is not connected"
    }
  }
}
