# Look up an environment by name
data "arcane_environment" "production" {
  name = "production"
}

# Look up an environment by ID
data "arcane_environment" "by_id" {
  id = "env-abc123"
}

# Use the environment data
output "environment_id" {
  value = data.arcane_environment.production.id
}

output "environment_uses_api_key" {
  value = data.arcane_environment.production.use_api_key
}
