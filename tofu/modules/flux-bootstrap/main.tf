# Amorçage GitOps commun aux environnements (ADR-0001) :
# Flux Operator + FluxInstance (repris ensuite par Flux) + Secrets de départ (gérés par OpenTofu).
#
# Les Secrets créés ici finissent dans l'état OpenTofu : l'état DOIT être chiffré (ADR-0007),
# ce que chaque environnement impose avec `encryption { ... enforced = true }`.

locals {
  repo_root = abspath("${path.module}/../../..")

  instance_yaml       = file("${local.repo_root}/cluster/base/flux-system/flux-instance.yaml")
  operator_values     = file("${local.repo_root}/cluster/base/flux-system/flux-operator-values.yaml")
  operator_chart_vers = regex("tag: \"([0-9.]+)\"", file("${local.repo_root}/cluster/base/flux-system/flux-operator.yaml"))[0]

  ghcr_dockerconfigjson = jsonencode({
    auths = {
      "ghcr.io" = {
        username = var.ghcr_username
        password = var.ghcr_token
        auth     = base64encode("${var.ghcr_username}:${var.ghcr_token}")
      }
    }
  })

  secrets = [
    {
      apiVersion = "v1"
      kind       = "Secret"
      metadata   = { name = "sops-age" }
      type       = "Opaque"
      stringData = { "age.agekey" = file(pathexpand(var.age_key_file)) }
    },
    {
      # GitHub App « platform-flux » : lecture du dépôt infra et des PR des applications.
      apiVersion = "v1"
      kind       = "Secret"
      metadata   = { name = "github-app" }
      type       = "Opaque"
      stringData = {
        githubAppID             = tostring(var.github_app_id)
        githubAppInstallationID = tostring(var.github_app_installation_id)
        githubAppPrivateKey     = file(pathexpand(var.github_app_private_key_file))
      }
    },
    {
      # Lecture GHCR : PAT classique read:packages (exception documentée, ADR-0015).
      apiVersion = "v1"
      kind       = "Secret"
      metadata   = { name = "ghcr-auth" }
      type       = "kubernetes.io/dockerconfigjson"
      stringData = { ".dockerconfigjson" = local.ghcr_dockerconfigjson }
    },
  ]
}

module "flux_operator_bootstrap" {
  source  = "controlplaneio-fluxcd/flux-operator-bootstrap/kubernetes"
  version = "0.10.0"

  revision = var.revision

  gitops_resources = {
    # ${CLUSTER_ENV} dans sync.path est résolu par le Job d'amorçage (runtime_info),
    # puis par la Kustomization racine (substituteFrom flux-runtime-info).
    instance_yaml = local.instance_yaml
    operator_chart = {
      version     = local.operator_chart_vers
      values_yaml = local.operator_values
    }
  }

  managed_resources = {
    secrets_yaml = join("\n---\n", [for s in local.secrets : yamlencode(s)])
    runtime_info = {
      labels = {
        "reconcile.fluxcd.io/watch" = "Enabled"
        "toolkit.fluxcd.io/runtime" = "true"
      }
      # Valeurs minimales connues d'OpenTofu ; le reste vient de cluster/<env>/runtime-info.yaml (SSA Merge).
      data = {
        CLUSTER_ENV  = var.cluster_env
        CLUSTER_NAME = var.cluster_name
      }
    }
  }
}
