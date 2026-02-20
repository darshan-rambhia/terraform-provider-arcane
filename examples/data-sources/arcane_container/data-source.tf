data "arcane_container" "postgres" {
  environment_id = arcane_environment.production.id
  name           = "postgres"
}

output "postgres_status" {
  value = data.arcane_container.postgres.status
}
