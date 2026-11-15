# Secrets : uniquement par TF_VAR_* ou par chemins de fichiers locaux. Jamais dans un .tfvars versionné.

variable "state_passphrase" {
  description = "Phrase de chiffrement de l'état (≥ 16 caractères), via TF_VAR_state_passphrase."
  type        = string
  sensitive   = true
  validation {
    condition     = length(var.state_passphrase) >= 16
    error_message = "La phrase de chiffrement doit faire au moins 16 caractères."
  }
}

variable "hcloud_token" {
  description = "Jeton API Hetzner Cloud (lecture/écriture, projet dédié)."
  type        = string
  sensitive   = true
}

variable "cluster_name" {
  type    = string
  default = "platform"
}

variable "location" {
  description = "Datacenter Hetzner (et région Object Storage) : fsn1, nbg1 ou hel1."
  type        = string
  default     = "fsn1"
}

variable "network_region" {
  type    = string
  default = "eu-central"
}

variable "server_type" {
  description = "cx33 (x86, 8 Go) ou cax21 (ARM, 8 Go). Vérifier prix et disponibilité avant d'appliquer."
  type        = string
  default     = "cx33"
  validation {
    condition     = contains(["cx33", "cax21", "cx43", "cax31"], var.server_type)
    error_message = "Types prévus : cx33, cax21 (8 Go) ; cx43, cax31 (16 Go)."
  }
}

variable "node_count" {
  description = "Nombre de nœuds control-plane (1 maintenant, 3 pour la haute disponibilité)."
  type        = number
  default     = 1
  validation {
    condition     = contains([1, 3], var.node_count)
    error_message = "etcd exige un nombre impair de nœuds : 1 ou 3."
  }
}

variable "k3s_version" {
  description = "Version k3s exacte à l'installation (les montées ensuite passent par SUC/Flux)."
  type        = string
  default     = "v1.35.10+k3s1"
}

variable "automatically_upgrade_os" {
  description = "Mises à jour transactionnelles de l'OS (redémarrages encadrés par Kured). Compromis ADR-0013."
  type        = bool
  default     = true
}

variable "ssh_public_key_file" {
  type    = string
  default = "~/.ssh/platform_hetzner_ed25519.pub"
}

variable "ssh_private_key_file" {
  description = "null pour utiliser ssh-agent (clé protégée par phrase de passe)."
  type        = string
  default     = null
  sensitive   = true
}

variable "tailscale_auth_key" {
  description = "Clé d'authentification Tailscale réutilisable, taguée tag:platform-node."
  type        = string
  sensitive   = true
}

variable "tailscale_magicdns_domain" {
  description = "Domaine MagicDNS du tailnet, ex. tail1234.ts.net."
  type        = string
}

variable "bucket_prefix" {
  description = "Préfixe des buckets (unique au niveau de Hetzner Object Storage)."
  type        = string
}

variable "s3_access_key" {
  type      = string
  sensitive = true
}

variable "s3_secret_key" {
  type      = string
  sensitive = true
}

variable "bootstrap_revision" {
  type    = number
  default = 1
}

variable "age_key_file" {
  description = "Clé privée age de l'environnement hetzner (distincte de la clé locale)."
  type        = string
  default     = "~/.config/sops/age/hetzner.txt"
  sensitive   = true
}

variable "github_app_id" {
  type = number
}

variable "github_app_installation_id" {
  type = number
}

variable "github_app_private_key_file" {
  type      = string
  sensitive = true
}

variable "ghcr_username" {
  type = string
}

variable "ghcr_token" {
  type      = string
  sensitive = true
}
