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
| 3. Surcouche Hetzner + OpenTofu | ✅ 2026-11-15 | `tofu validate` OK (local et hetzner), validation hetzner : 105 ressources kubeconform |
| 4. app-template | ✅ 2026-11-15 | `make test` vert dans app-template ; images construites en local, 0 CVE HIGH/CRITICAL corrigeable |
| 5. Pipelines + validation | ⏸ commencée (arrêt volontaire, quota) | voir « Reprise » ci-dessous |
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

## Reprise (arrêt du 2026-11-15, phase 5 commencée)
**Bug trouvé, à corriger en premier** : `cluster/base/controllers/observability/otel-values.yaml` n'est **pas un YAML valide** (ligne 78, colonne 79 : « mapping values are not allowed here »). Les instructions OTTL non quotées contiennent `: `. Il faut mettre chaque instruction de `log_statements` et `trace_statements` entre quotes simples, en doublant les `'` internes. `make test` ne le détectait pas, parce que le fichier passe par un configMapGenerator sans être analysé. Il faut ajouter une analyse YAML de ce fichier à `scripts/validate.sh`.

Fait en phase 5 (non commité au moment de l'arrêt, puis commité en « WIP ») :
- `scripts/install-tools.sh` : actionlint 1.7.12 et cosign 3.1.4 ajoutés.
- `scripts/test-redaction.sh` et `tests/redaction/{input.jsonl,check.py}` : test du masquage avec le vrai collecteur (docker, image otel/opentelemetry-collector-contrib:0.163.2). Jamais passé avec succès, à cause du bug YAML ci-dessus. Ajouter `tests/redaction/` à l'allowlist de `.gitleaks.toml` si Gitleaks signale les faux jetons.

SHA des actions relevés (à figer dans les workflows) :
actions/checkout v7.0.1 3d3c42e5aac5ba805825da76410c181273ba90b1 · astral-sh/setup-uv v10.3.3 68559e99cc807c1eb7e9238fd43a15bba460eec2 · docker/setup-buildx-action v4.4.1 f87e5991a6d7451dcb8d9637bfbc97413f497069 · docker/setup-qemu-action v4.4.0 99012661954931238ded8c8b007157a8430204e1 · docker/login-action v4.8.0 8aa5831fb8d48d8f76594c7ddde61ae58019f550 · sigstore/cosign-installer v4.1.3 756fe2d284c7292c4e9b62c8a28f1e5629c59532 · actions/create-github-app-token v3.3.0 64ab729a6294000f5aa4a4bc0b73d1f32bc45510 · actions/github-script v9.0.0 3a2844b7e9c422d3c10d287c895573f7108da1b3 · renovatebot/github-action v46.3.13 266fef21329c44765ad42f9dea1f01f5d2d91084

Reste à faire en phase 5 :
1. Workflows `infra` : `validate.yml` (make test + test-redaction + actionlint), `tofu-plan.yml` (garde-fou des destructions → environnement `hetzner-destroy`), `k3s-minor-guard`, `renovate.yml` + `renovate.json` (commentaires `# renovate:` dans install-tools.sh), `weekly-maintenance.yml`.
2. Workflows réutilisables `app-ci.yml` (tests, scans, migration-guard → environnement `destructive-migration`, build multi-arch, publish.sh SIGN=true, trivy image, commentaire de l'URL de préversion), et `app-template/.github/workflows/ci.yml` qui les appelle.
3. alert-bridge (Go, image, manifestes) ; slo-sync (écrit plus tard, ou doc de création manuelle des alertes).
4. Notifications Flux vers GitHub (statuts de commit).
Puis phase 6 : runbooks, secrets-de-depart.md, reglages-github.md, CLAUDE.md d'infra, rapport final.

Note sur l'environnement : `tofu validate`, docker et kubectl doivent tourner hors du bac à sable de Claude Code (sockets locales).
