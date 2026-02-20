# Create an Arcane environment
resource "arcane_environment" "production" {
  name        = "production"
  description = "Production Docker environment"
  use_api_key = true
}

# Create a development environment without API key
resource "arcane_environment" "development" {
  name        = "development"
  description = "Development Docker environment"
  use_api_key = false
}

# Output the access token (only available when use_api_key = true)
output "production_access_token" {
  value     = arcane_environment.production.access_token
  sensitive = true
}
