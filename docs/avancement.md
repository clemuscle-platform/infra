# Avancement

Ce fichier est mis à jour à chaque étape. Une nouvelle session commence par le lire, puis par `git log --oneline` dans `infra` et dans `app-template`.

## Contexte fixe
- Dépôts locaux : `~/plateforme/infra` et `~/plateforme/app-template`. Pas de remote, et rien n'est poussé sans l'accord de l'utilisateur.
- Interdits : apply, bootstrap, `helm install`, push, sudo, secrets en clair. kubectl ne sert qu'à lire.
- Le bac à sable de Claude Code bloque 127.0.0.1:6443. Les lectures kubectl se font donc hors bac à sable.
- Organisation GitHub provisoire : `clemuscle-platform`, à créer par l'utilisateur. Domaine provisoire : `example.com`.
- Clé age locale (publique) : `age18enzprddd0at7gzk4929j4x093u3z52lvmcjv308yua2clfsh5zqx46utu`. La clé Hetzner viendra plus tard.

## Phases
| Phase | État | Notes |
|---|---|---|
| 0. Inventaire | ✅ 2026-11-15 | outils figés dans `~/.local/bin` (`scripts/install-tools.sh`), make extrait du .deb Debian ; k3s passé en v1.35.10 par l'utilisateur |
| 1. Architecture | ✅ validée le 2026-11-15 | `docs/architecture.md`, `docs/decisions/0001` à `0017` (révisées en phase 2 : 0002, 0006, 0010, 0011, 0015) |
| 2. Base + surcouche locale | ✅ 2026-11-15 | `make test` vert : validation local (92 ressources kubeconform), 16 tests VAP, RBAC statique, Trivy, Gitleaks |
| 3. Surcouche Hetzner + OpenTofu | à faire | |
| 4. app-template | à faire | |
| 5. Pipelines + validation | à faire | |
| 6. Runbooks + documentation | à faire | |

## État du k3s local (2026-11-15)
- v1.35.10+k3s1, WSL2, SQLite, Traefik, servicelb, local-path, metrics-server.
- Releases Helm installées à la main : cert-manager v1.21.3 (Flux la reprendra), kube-prometheus-stack, sealed-secrets, kagent, nvidia-device-plugin (projet LLM, que Flux ne touche pas), gitlab-runner (à supprimer par l'utilisateur).
- Espaces de noms orphelins à supprimer par l'utilisateur : `app`, `app-demo`, `ci`.

## Décisions ouvertes
- ADR-0017 : R2 ou B2, GlitchTip ou Sentry, Brevo ou Resend.
- Nom définitif de l'organisation et du domaine.
- ADR-0013 : mises à jour automatiques de l'OS sur un seul nœud, un compromis à valider.

## Phase 2 : ce qui est en place
- `cluster/base/flux-system` : FluxInstance auto-gérée (Flux 2.10.0, multi-tenant, substitution stricte) et Flux Operator via ResourceSet.
- `cluster/base/platform/layers.yaml` : couches controllers → configs → policies → tenants → apps.
- Contrôleurs : cert-manager v1.21.3 (reprend la release existante), CNPG 1.30.1 (chart 0.29.1) avec plugin Barman Cloud 0.16.0 (chart 0.9.0), OpenObserve 1.1.2 (chart 1.1.6), OTel Collector (chart 0.178.3, contrib) avec masquage.
- Configs : rôle `agent-reader`, rôle `platform-tenant-deployer`.
- `cluster/base/policies` : 6 VAP et leurs bindings, avec tests kyverno (les cas CNPG ne sont pas testables hors cluster).
- `cluster/base/tenants/app-tenants.yaml` : un ResourceSet unique pour les niveaux preview, staging et prod.
- `cluster/local` : variables, Garage v2.5.0 avec tâche d'initialisation par API, CA auto-signée, secrets SOPS générés par `scripts/gen-local-secrets.sh`.
- `cluster/apps/hello.yaml` : déclaration d'exemple.

## Écrit mais NON vérifié sur un cluster
- Reprise de la release cert-manager existante par helm-controller.
- Tâche d'initialisation de Garage : l'API v2 a été vérifiée dans la spécification OpenAPI, pas à l'exécution.
- Syntaxe OTTL et regex du collecteur : test prévu en phase 5, avec le binaire du collecteur.
- Règles VAP sur les `Cluster` CNPG et protection contre la suppression en production : non testables avec la CLI kyverno.
- NetworkPolicy des instances CNPG vers l'API (port 6443) avec kube-router.
- Comportement de `copyFrom` pour `ghcr-auth` et `platform-s3`.

## Reste à faire (repris dans les phases suivantes)
- alert-bridge (code + image) : phase 5.
- Notifications Flux vers GitHub (statut de commit, commentaire de PR) : phase 5.
- NetworkPolicy des espaces de noms de la plateforme.
- Test du masquage OTel : phase 5.
