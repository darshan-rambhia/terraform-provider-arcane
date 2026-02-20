# Complete example showing typical Arcane provider usage
#
# This example demonstrates:
# - Creating an environment
# - Looking up existing projects
# - Deploying projects

terraform {
  required_providers {
    arcane = {
      source  = "darshan-raul/arcane"
      version = "~> 0.1"
    }
  }
}

# Variables
variable "arcane_url" {
  description = "Arcane API URL"
  type        = string
  default     = "http://arcane.homelab.local:8000"
}

variable "arcane_api_key" {
  description = "Arcane API key (optional)"
  type        = string
  default     = ""
  sensitive   = true
}

# Provider configuration
provider "arcane" {
  url     = var.arcane_url
  api_key = var.arcane_api_key
}

# Create an environment for homelab
resource "arcane_environment" "homelab" {
  name        = "homelab"
  description = "Homelab Docker environment managed by Terraform"
  use_api_key = true
}

# Look up existing projects in the environment
# Note: Projects are auto-discovered from docker compose stacks on the Docker host
data "arcane_project" "monitoring" {
  environment_id = arcane_environment.homelab.id
  name           = "monitoring"
}

data "arcane_project" "traefik" {
  environment_id = arcane_environment.homelab.id
  name           = "traefik"
}

# Deploy the monitoring stack
resource "arcane_project_deployment" "monitoring" {
  environment_id = arcane_environment.homelab.id
  project_id     = data.arcane_project.monitoring.id

  # Always pull latest images
  pull = true
}

# Deploy the traefik stack
resource "arcane_project_deployment" "traefik" {
  environment_id = arcane_environment.homelab.id
  project_id     = data.arcane_project.traefik.id

  # Pull images and force recreate on deploy
  pull           = true
  force_recreate = true
}

# Outputs
output "environment_id" {
  description = "The ID of the created environment"
  value       = arcane_environment.homelab.id
}

output "environment_access_token" {
  description = "The access token for the environment"
  value       = arcane_environment.homelab.access_token
  sensitive   = true
}

output "monitoring_status" {
  description = "Status of the monitoring deployment"
  value       = arcane_project_deployment.monitoring.status
}

output "traefik_status" {
  description = "Status of the traefik deployment"
  value       = arcane_project_deployment.traefik.status
}
