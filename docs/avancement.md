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
| 1. Architecture | ⏳ en attente de validation | `docs/architecture.md`, `docs/decisions/0001` à `0017` |
| 2. Base + surcouche locale | à faire | |
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
