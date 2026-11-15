# Fiches de décision (ADR)

Une fiche par décision d'architecture, au format : **Statut · Contexte · Décision · Alternatives écartées · Conséquences**.

Statuts : `proposée` → `acceptée` → éventuellement `remplacée par NNNN`. On ne modifie pas une fiche acceptée : on en écrit une nouvelle qui la remplace.

| N° | Décision | Statut |
|---|---|---|
| [0001](0001-gitops-flux-operator.md) | GitOps avec Flux Operator, amorcé par OpenTofu | proposée |
| [0002](0002-livraison-applications-oci.md) | Livraison des applications par artefacts OCI signés, trois niveaux | proposée |
| [0003](0003-environnements-base-surcouches.md) | Une base, deux surcouches (`local`, `hetzner`) | proposée |
| [0004](0004-composants-distribution.md) | Réutiliser Traefik, servicelb, local-path ; Ingress plutôt que Gateway API | proposée |
| [0005](0005-admission-vap.md) | ValidatingAdmissionPolicy + Pod Security Admission plutôt que Kyverno | proposée |
| [0006](0006-isolation-applications.md) | Isolation : espace de noms par application et niveau | proposée |
| [0007](0007-secrets-sops-age.md) | Secrets SOPS/age, une clé par environnement, état OpenTofu chiffré | proposée |
| [0008](0008-stockage-objet-sauvegardes.md) | Garage en local, Hetzner Object Storage + copie hors hébergeur | proposée |
| [0009](0009-postgresql-cnpg.md) | PostgreSQL CloudNativePG avec le plugin Barman Cloud | proposée |
| [0010](0010-observabilite.md) | OpenTelemetry → OpenObserve, masquage dans le collecteur | proposée |
| [0011](0011-dns-tls.md) | Cloudflare + external-dns + cert-manager DNS-01 | proposée |
| [0012](0012-acces-prive-tailscale.md) | Accès privé par Tailscale, seuls 80/443 publics | proposée |
| [0013](0013-entretien.md) | Entretien : Renovate, SUC, Kured, instantanés etcd | proposée |
| [0014](0014-agents-acces-autonomie.md) | Agents : lecture seule, écriture par Git, règles d'autonomie, alertes → tickets | proposée |
| [0015](0015-ci-github.md) | CI GitHub Actions, workflows réutilisables, GitHub App | proposée |
| [0016](0016-hetzner-kube-hetzner.md) | Cible Hetzner avec kube-hetzner, nœud unique extensible | proposée |
| [0017](0017-services-externes.md) | Services externes : sonde, erreurs, e-mail | proposée (choix ouverts) |
