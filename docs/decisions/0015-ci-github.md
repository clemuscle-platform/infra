# 0015 — CI GitHub Actions, workflows réutilisables, GitHub App

**Statut** : proposée · 2026-11-15

## Décision
- Organisation GitHub : **`clemuscle-platform`** (nom provisoire, disponible le 2026-11-15, que l'utilisateur doit créer). Dépôts : `infra` (privé), `app-template` (privé, marqué *template*), puis un dépôt par application.
- Les **workflows réutilisables** sont dans `infra/.github/workflows/` (`app-ci.yml`, `app-release.yml`, `app-preview-cleanup.yml`). Les dépôts d'applications les appellent avec une référence figée par SHA (`uses: clemuscle-platform/infra/.github/workflows/app-ci.yml@<sha>`), mise à jour par Renovate. Réglage requis : *Actions > Access > Accessible from repositories in the organization*.
- Contrôles de la CI `infra` : `tofu fmt -check`, `tofu validate`, `kustomize build` par surcouche, `kubeconform` (avec les schémas des CRD), `flux build` / `flux-operator build`, `kyverno test` (VAP), test du masquage OTel, `trivy config`, `gitleaks`, garde-fous (base sans valeur d'environnement, Secrets chiffrés, `tofu-destroy-guard`).
- Contrôles de la CI d'application : tests, lint, `semgrep` (règles `p/default` + règles maison), `gitleaks`, `trivy fs` puis `trivy image` (échec sur HIGH/CRITICAL corrigeables), construction multi-architecture, SBOM, signature cosign, `migration-guard`.
- Toutes les actions sont figées par SHA. `permissions:` est minimal par job, avec `contents: read` par défaut.
- Les jetons proviennent de GitHub Apps (`actions/create-github-app-token`), jamais de PAT. GHCR utilise `GITHUB_TOKEN` (`packages: write`).

## Réglages GitHub recommandés
Ils sont détaillés dans `docs/reglages-github.md` : règles de branche (rulesets) au niveau de l'organisation, contrôles requis, signatures exigées, interdiction des force-push, CODEOWNERS, environnements `production` et `hetzner` avec relecteurs requis pour `tofu apply`, rotation des clés des GitHub Apps tous les 90 jours.
