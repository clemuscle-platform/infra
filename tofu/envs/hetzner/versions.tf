terraform {
  required_version = ">= 1.13.0"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.70.2"
    }
    aws = {
      # Utilisé uniquement pour l'API S3 de Hetzner Object Storage (buckets).
      source  = "hashicorp/aws"
      version = "6.73.0"
    }
    http = {
      source  = "hashicorp/http"
      version = "3.6.3"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "3.3.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "3.5.0"
    }
  }

  # État sur Hetzner Object Storage (bucket <prefix>-tfstate, créé à la main AVANT le premier init,
  # voir docs/runbooks/migration-hetzner.md). Configuration partielle : tofu init -backend-config=backend.hcl
  backend "s3" {}

  # État chiffré côté client (il contient la clé age, les jetons API et le kubeconfig). ADR-0007.
  encryption {
    key_provider "pbkdf2" "state" {
      passphrase = var.state_passphrase
    }
    method "aes_gcm" "state" {
      keys = key_provider.pbkdf2.state
    }
    state {
      method   = method.aes_gcm.state
      enforced = true
    }
    plan {
      method   = method.aes_gcm.state
      enforced = true
    }
  }
}
