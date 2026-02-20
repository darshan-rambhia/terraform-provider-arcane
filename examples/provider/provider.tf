# Configure the Arcane provider
terraform {
  required_providers {
    arcane = {
      source  = "darshan-raul/arcane"
      version = "~> 0.1"
    }
  }
}

# Provider configuration
provider "arcane" {
  # Arcane API URL (can also be set via ARCANE_URL environment variable)
  url = "http://arcane.homelab.local:8000"

  # API key for authentication (can also be set via ARCANE_API_KEY environment variable)
  # api_key = var.arcane_api_key
}
