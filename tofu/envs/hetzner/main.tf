# Environnement « hetzner » (ADR-0016) : serveur(s), pare-feu, k3s (kube-hetzner), buckets S3, amorçage Flux.
# Tout le reste est installé par Flux depuis cluster/hetzner.
# Lancement (humain) : make plan ENV=hetzner, relire le plan (contrôle tofu-destroy-guard), make deploy ENV=hetzner.

provider "hcloud" {
  token = var.hcloud_token
}

# Plages IP de Cloudflare : Traefik ne fait confiance qu'à elles pour X-Forwarded-For (IP réelle du client).
data "http" "cloudflare_ipv4" {
  url = "https://www.cloudflare.com/ips-v4"
}

data "http" "cloudflare_ipv6" {
  url = "https://www.cloudflare.com/ips-v6"
}

locals {
  cloudflare_ips = concat(
    split("\n", trimspace(data.http.cloudflare_ipv4.response_body)),
    split("\n", trimspace(data.http.cloudflare_ipv6.response_body)),
  )
  s3_endpoint_host = "${var.location}.your-objectstorage.com"
}

module "kube_hetzner" {
  source  = "kube-hetzner/kube-hetzner/hcloud"
  version = "3.2.1"

  providers = {
    hcloud = hcloud
  }

  hcloud_token    = var.hcloud_token
  ssh_public_key  = file(pathexpand(var.ssh_public_key_file))
  ssh_private_key = var.ssh_private_key_file == null ? null : file(pathexpand(var.ssh_private_key_file))
  cluster_name    = var.cluster_name
  network_region  = var.network_region

  # Nœud unique (control-plane qui accepte les charges) ; 3 nœuds plus tard via var.node_count.
  control_plane_nodepools = [
    {
      name        = "cp"
      server_type = var.server_type
      location    = var.location
      labels      = []
      taints      = []
      count       = var.node_count
    }
  ]
  agent_nodepools = []

  k3s_version                      = var.k3s_version
  automatically_upgrade_kubernetes = false # versions figées dans Git, montées par SUC (Flux)
  automatically_upgrade_os         = var.automatically_upgrade_os
  enable_system_upgrade_controller = false # installé par Flux (cluster/hetzner)
  enable_kured                     = false # installé par Flux (cluster/hetzner)
  enable_cert_manager              = false # installé par Flux (cluster/base)
  enable_secrets_encryption        = true
  create_kubeconfig                = false

  ingress_controller = "traefik"
  traefik_merge_values = yamlencode({
    ports = {
      web = {
        forwardedHeaders = { trustedIPs = local.cloudflare_ips }
        redirections = {
          entryPoint = { to = "websecure", scheme = "https", permanent = true }
        }
      }
      websecure = {
        forwardedHeaders = { trustedIPs = local.cloudflare_ips }
      }
    }
  })

  # Accès privé : SSH et API Kubernetes uniquement via Tailscale ; seuls 80/443 restent publics.
  node_transport_mode      = "tailscale"
  firewall_ssh_source      = null
  firewall_kube_api_source = null
  tailscale_auth_key       = var.tailscale_auth_key
  tailscale_node_transport = {
    bootstrap_mode  = "cloud_init"
    magicdns_domain = var.tailscale_magicdns_domain
    auth = {
      mode = "auth_key"
    }
    routing = {
      advertise_node_private_routes = false
    }
  }

  etcd_s3_backup = {
    "etcd-s3-endpoint"            = local.s3_endpoint_host
    "etcd-s3-access-key"          = var.s3_access_key
    "etcd-s3-secret-key"          = var.s3_secret_key
    "etcd-s3-bucket"              = aws_s3_bucket.platform["etcd"].bucket
    "etcd-s3-region"              = var.location
    "etcd-snapshot-schedule-cron" = "0 */6 * * *"
    "etcd-snapshot-retention"     = "28"
  }
}

provider "kubernetes" {
  host                   = module.kube_hetzner.kubeconfig_data.host
  client_certificate     = module.kube_hetzner.kubeconfig_data.client_certificate
  client_key             = module.kube_hetzner.kubeconfig_data.client_key
  cluster_ca_certificate = module.kube_hetzner.kubeconfig_data.cluster_ca_certificate
}

provider "helm" {
  kubernetes = {
    host                   = module.kube_hetzner.kubeconfig_data.host
    client_certificate     = module.kube_hetzner.kubeconfig_data.client_certificate
    client_key             = module.kube_hetzner.kubeconfig_data.client_key
    cluster_ca_certificate = module.kube_hetzner.kubeconfig_data.cluster_ca_certificate
  }
}

module "flux" {
  source     = "../../modules/flux-bootstrap"
  depends_on = [module.kube_hetzner]

  cluster_env  = "hetzner"
  cluster_name = var.cluster_name
  revision     = var.bootstrap_revision

  age_key_file                = var.age_key_file
  github_app_id               = var.github_app_id
  github_app_installation_id  = var.github_app_installation_id
  github_app_private_key_file = var.github_app_private_key_file
  ghcr_username               = var.ghcr_username
  ghcr_token_file             = var.ghcr_token_file
}
