# Avancement

Ce fichier est mis à jour à chaque étape. Une nouvelle session commence par le lire, puis par `git log --oneline` dans `infra` et dans `app-template`.

## Contexte fixe
- Dépôts : `~/plateforme/infra` (poussé sur `clemuscle-platform/infra`, privé) et `~/plateforme/app-template` (local uniquement, pas de dépôt distant sans accord).
- Autorisé : push sur `infra`, `tofu apply` sur `tofu/envs/local`, lecture et réconciliation Flux sur le k3s local.
- Interdit sans accord explicite : Hetzner ou toute dépense, sudo, lecture de `~/.config/sops`, création de dépôt pour une application de test.
- Le bac à sable de Claude Code bloque 127.0.0.1:6443 : kubectl, flux, tofu et docker tournent hors bac à sable.
- Clé age locale (publique) : `age18enzprddd0at7gzk4929j4x093u3z52lvmcjv308yua2clfsh5zqx46utu`. La clé Hetzner viendra plus tard.

## Phases
| Phase | État | Notes |
|---|---|---|
| 0. Inventaire | ✅ 2026-11-15 | outils figés dans `~/.local/bin` (`scripts/install-tools.sh`), make extrait du .deb Debian ; k3s passé en v1.35.10 par l'utilisateur |
| 1. Architecture | ✅ validée le 2026-11-15 | `docs/architecture.md`, `docs/decisions/0001` à `0017` (révisées en phase 2 : 0002, 0006, 0010, 0011, 0015) |
| 2. Base + surcouche locale | ✅ 2026-11-15 | `make test` vert : validation local (92 ressources kubeconform), 16 tests VAP, RBAC statique, Trivy, Gitleaks |
| 3. Surcouche Hetzner + OpenTofu | ✅ 2026-11-15 | `tofu validate` OK (local et hetzner), validation hetzner : 105 ressources kubeconform |
| 4. app-template | ✅ 2026-11-15 | `make test` vert dans app-template ; images construites en local, 0 CVE HIGH/CRITICAL corrigeable |
| 5. Pipelines + validation | 🟡 presque finie | CI verte sur GitHub, Flux déployé sur le k3s local le 2026-11-16 ; reste : voir « Reprise » |
| 6. Runbooks + documentation | à faire | |

## État du k3s local (2026-11-15, avant Flux)
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

## Phase 3 : ce qui est en place
- `tofu/modules/flux-bootstrap` : module officiel `flux-operator-bootstrap` 0.10.0. Il crée les Secrets de départ (sops-age, github-app, ghcr-auth) et la ConfigMap runtime_info (CLUSTER_ENV, CLUSTER_NAME).
- `tofu/envs/local` : amorçage seul sur le k3s existant, état local chiffré (pbkdf2 + aes_gcm, `enforced`).
- `tofu/envs/hetzner` : kube-hetzner 3.2.1 (un nœud, Tailscale, etcd vers S3, SUC, Kured et cert-manager du module désactivés), 4 buckets avec `prevent_destroy`, backend S3 Hetzner, état chiffré.
- `cluster/hetzner` : external-dns (Cloudflare, upsert-only), SUC v0.21.1 (Git amont + correction de l'image), Kured 1.23, opérateur Tailscale, OpenObserve sur S3, ClusterIssuers Let's Encrypt (DNS-01), Plan k3s figé, copie quotidienne vers R2 avec rclone.
- Secrets Hetzner : valeurs factices `A-REMPLACER`, chiffrées pour l'instant avec la clé LOCALE (`scripts/gen-hetzner-placeholders.sh`).

## Phase 3 : écrit mais non vérifié
- `tofu plan` et `tofu apply` (jamais lancés : il faudrait les identifiants Hetzner).
- kube-hetzner : 80/443 restent toujours ouverts à tout Internet (code du module vérifié). Le filtrage Cloudflare côté Traefik reste à décider (ADR-0011).
- Sentinelle de redémarrage de Kured sur Leap Micro (`/var/run/reboot-required`).
- `ZO_S3_*` d'OpenObserve vers Hetzner Object Storage.
- Le Public Access Block de Hetzner Object Storage (exception Trivy, à vérifier à la main).

## Reprise (état au 2026-11-21)
**Sur le k3s local** (Flux 2.10.0, source `clemuscle-platform/infra` lue via la GitHub App `clemuscle-platform-flux`, en lecture seule) :
- Prêtes : flux-system, platform-controllers, platform-configs, platform-policies, platform-tenants, platform-ops, apps.
- HelmReleases : cert-manager (release existante reprise), CloudNativePG + plugin Barman, Flux Operator, OpenObserve, collecteur OTel. Garage tourne, son initialisation est passée.
- alert-bridge tourne dans `platform-ops`.
- `app-hello-staging` et `app-hello-prod` échouent, c'est attendu : l'artefact `ghcr.io/clemuscle-platform/hello-deploy` n'existe pas tant que `app-template` n'a pas de dépôt.

**Vérifié sur le cluster depuis la phase 2** : reprise de cert-manager, initialisation de Garage (après correctif), `copyFrom` dans un ResourceSet. Piège : `copyFrom` n'est interprété que par Flux Operator, jamais dans un Secret appliqué par kustomize-controller (corrigé pour `platform-ops` le 2026-11-21).

**slo-sync (fait le 2026-11-21)** : CronJob dans `platform-observability`, image `slo-sync:0.1.0` publiée et signée par la CI. Vérifié de bout en bout sur le k3s local : 4 alertes créées pour une application de test, alerte rapide déclenchée par OpenObserve et reçue par alert-bridge, second passage sans changement, alertes retirées à la suppression de l'application. Conception et points vérifiés : ADR-0010.
Pièges : le garde-fou SSRF d'OpenObserve bloque les destinations internes (exception `ZO_SSRF_ALLOWED_HOSTS`) ; le chart ne redémarre pas OpenObserve quand sa configuration change.

**Reste à faire en phase 5 :**
1. Notifications Flux vers GitHub (statuts de commit) : la GitHub App est en lecture seule, il faudra lui ajouter « Commit statuses: write » (clic de l'utilisateur).
2. ~~Test de bout en bout d'une vraie application~~ : fait le 2026-11-21 avec `hello` (dépôts `app-template`, modèle, et `hello` créés ; App `clemuscle-platform-ci`). Trois bugs du modèle corrigés en route : `.infra/` analysé par le lint et les scans, image non remplacée par son digest dans l'artefact (garde-fou ajouté à `publish.sh`), cache de kubeconform absent sur un runner neuf. La production de `hello` attend une étiquette `vX.Y.Z`.
3. Renovate : créer la GitHub App `platform-renovate`, puis la variable `RENOVATE_APP_CLIENT_ID` et le secret `RENOVATE_APP_PRIVATE_KEY` (le job est ignoré d'ici là).

**Ménage côté utilisateur** : supprimer les namespaces `app`, `app-demo`, `ci` et la release `gitlab-runner`.

Puis phase 6 : runbooks, secrets-de-depart.md, reglages-github.md, CLAUDE.md d'infra, rapport final.
