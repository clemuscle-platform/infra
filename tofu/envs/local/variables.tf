variable "state_passphrase" {
  description = "Phrase de chiffrement de l'état (≥ 16 caractères). Fournir par TF_VAR_state_passphrase, jamais dans un fichier."
  type        = string
  sensitive   = true
  validation {
    condition     = length(var.state_passphrase) >= 16
    error_message = "La phrase de chiffrement doit faire au moins 16 caractères."
  }
}

variable "kubeconfig_path" {
  description = "Kubeconfig administrateur du k3s local."
  type        = string
  default     = "~/.kube/config"
}

variable "kube_context" {
  description = "Contexte kubectl du k3s local."
  type        = string
  default     = "default"
}

variable "bootstrap_revision" {
  description = "Incrémenter pour relancer l'amorçage."
  type        = number
  default     = 1
}

variable "age_key_file" {
  description = "Clé privée age de l'environnement local."
  type        = string
  default     = "~/.config/sops/age/keys.txt"
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

variable "ghcr_token_file" {
  description = "Fichier local contenant le PAT classique read:packages."
  type        = string
  default     = "~/.config/platform/ghcr-read-token"
  sensitive   = true
}
