# Environnement « local » : le k3s du PC existe déjà ; OpenTofu ne fait que l'amorçage de Flux.
# Lancement (humain) : make plan ENV=local, puis make deploy ENV=local.

provider "kubernetes" {
  config_path    = pathexpand(var.kubeconfig_path)
  config_context = var.kube_context
}

provider "helm" {
  kubernetes = {
    config_path    = pathexpand(var.kubeconfig_path)
    config_context = var.kube_context
  }
}

module "flux" {
  source = "../../modules/flux-bootstrap"

  cluster_env  = "local"
  cluster_name = "local-wsl"
  revision     = var.bootstrap_revision

  age_key_file                = var.age_key_file
  github_app_id               = var.github_app_id
  github_app_installation_id  = var.github_app_installation_id
  github_app_private_key_file = var.github_app_private_key_file
  ghcr_username               = var.ghcr_username
  ghcr_token_file             = var.ghcr_token_file
}
