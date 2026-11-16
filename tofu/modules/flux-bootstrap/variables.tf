variable "cluster_env" {
  description = "Nom de la surcouche (dossier cluster/<env>) : local ou hetzner."
  type        = string
  validation {
    condition     = contains(["local", "hetzner"], var.cluster_env)
    error_message = "cluster_env doit valoir local ou hetzner."
  }
}

variable "cluster_name" {
  description = "Nom du cluster (étiquettes, propriétaire external-dns)."
  type        = string
}

variable "revision" {
  description = "Incrémenter pour relancer le Job d'amorçage sans autre changement."
  type        = number
  default     = 1
}

variable "age_key_file" {
  description = "Chemin LOCAL de la clé privée age de l'environnement (jamais dans Git)."
  type        = string
  sensitive   = true
}

variable "github_app_id" {
  description = "ID de la GitHub App platform-flux."
  type        = number
}

variable "github_app_installation_id" {
  description = "ID d'installation de la GitHub App platform-flux sur l'organisation."
  type        = number
}

variable "github_app_private_key_file" {
  description = "Chemin LOCAL de la clé privée PEM de la GitHub App platform-flux."
  type        = string
  sensitive   = true
}

variable "ghcr_username" {
  description = "Compte GitHub propriétaire du PAT de lecture GHCR."
  type        = string
}

variable "ghcr_token_file" {
  description = "Chemin LOCAL du PAT classique read:packages (lecture des images et artefacts GHCR)."
  type        = string
  sensitive   = true
}
