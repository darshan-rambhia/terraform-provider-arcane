resource "arcane_container_registry" "ghcr" {
  name      = "GitHub Container Registry"
  url       = "https://ghcr.io"
  auth_type = "basic"
  username  = "my-github-user"
  password  = var.ghcr_token
}

resource "arcane_container_registry" "dockerhub" {
  name = "Docker Hub"
  url  = "https://index.docker.io/v1/"
}
