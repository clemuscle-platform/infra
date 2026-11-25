# 0015 — CI GitHub Actions, workflows réutilisables, GitHub App

**Statut** : proposée · 2026-11-15

## Décision
- Organisation GitHub : **`clemuscle-platform`** (nom provisoire, disponible le 2026-11-15, que l'utilisateur doit créer). Dépôts : `infra` (privé), `app-template` (privé, marqué *template*), puis un dépôt par application.
- Les **workflows réutilisables** sont dans `infra/.github/workflows/` (`app-ci.yml`, `app-release.yml`, `app-preview-cleanup.yml`). Les dépôts d'applications les appellent avec une référence figée par SHA (`uses: clemuscle-platform/infra/.github/workflows/app-ci.yml@<sha>`), mise à jour par Renovate. Réglage requis : *Actions > Access > Accessible from repositories in the organization*.
- Contrôles de la CI `infra` : `tofu fmt -check`, `tofu validate`, `kustomize build` par surcouche, `kubeconform` (avec les schémas des CRD), `flux build` / `flux-operator build`, `kyverno test` (VAP), test du masquage OTel, `trivy config`, `gitleaks`, garde-fous (base sans valeur d'environnement, Secrets chiffrés, `tofu-destroy-guard`).
- Contrôles de la CI d'application : tests, lint, `semgrep` (règles `p/default` + règles maison), `gitleaks`, `trivy fs` puis `trivy image` (échec sur HIGH/CRITICAL corrigeables), construction multi-architecture, SBOM, signature cosign, `migration-guard`.
- Toutes les actions sont figées par SHA. `permissions:` est minimal par job, avec `contents: read` par défaut.
- Les jetons proviennent de GitHub Apps (`actions/create-github-app-token`), jamais de PAT. GHCR utilise `GITHUB_TOKEN` (`packages: write`) dans la CI.
- **Exception documentée (phase 2)** : GHCR n'accepte pas les jetons d'installation de GitHub App. Pour que le cluster **lise** les images et les artefacts privés (Secret `ghcr-auth`), il faut un **PAT classique** limité à `read:packages`, créé sur un compte machine ou sur le compte de l'utilisateur, avec une expiration de 90 jours et une rotation consignée dans `docs/secrets-de-depart.md`. Ce jeton ne donne aucun droit en écriture.

## Mise en œuvre (2026-11-21)
- GitHub App **`clemuscle-platform-ci`** : `contents: read` et `metadata: read` seulement, installée sur `infra` seul. La CI d'une application l'utilise pour récupérer `infra` (outils figés, règles d'admission) dans `.infra/`, que l'application exclut de son lint et de ses scans.
- **Plan Free** : les secrets d'organisation ne sont pas transmis aux dépôts privés. `PLATFORM_CI_CLIENT_ID` et `PLATFORM_CI_APP_PRIVATE_KEY` sont donc définis **sur chaque dépôt d'application** (étape du README d'app-template). La clé est conservée hors dépôt, dans `~/.config/platform/platform-ci-app.pem` (droits 600).
- `infra` autorise l'appel de ses workflows par les dépôts de l'organisation (Actions → Access : *organization*). Sans ce réglage, `app-ci.yml` est introuvable depuis une application.
- `actions/create-github-app-token` utilise `client-id` (l'entrée `app-id` est dépréciée).
- Validé de bout en bout avec `hello` : CI verte, artefact `main` publié, staging déployé par Flux avec l'image référencée par digest, traces dans OpenObserve, alertes SLO créées par slo-sync.

## Dépôts publics et garde-fous (2026-11-25)
- **Constat** : sur le plan Free, un dépôt **privé** n'a ni règles de branche ni environnements protégés. La fusion automatique de `hello` #1 a eu lieu 13 s après l'ouverture de la PR, avant la fin des tests, et l'approbation des migrations destructives n'était exigée par rien.
- **Décision** : les dépôts de l'organisation sont **publics** (gratuit, protections réelles). Historique vérifié par gitleaks avant la bascule ; adresses d'auteur remplacées par l'adresse `noreply` de GitHub. Les secrets restent chiffrés par SOPS, les paquets GHCR restent privés.
- **`scripts/github-guardrails.sh infra|app <dépôt>`** applique, de façon idempotente, les règles versionnées dans `github/rulesets/` :
  - `main` d'une application : PR obligatoire, contrôles `platform / test` et `platform / migration-gate` émis par GitHub Actions, squash seulement, ni suppression ni force-push ;
  - `main` d'`infra` : PR, relecture du CODEOWNER (l'humain) et contrôles `validate`, `k3s-gate`. Les administrateurs de l'organisation peuvent fusionner une PR sans relecture, jamais pousser directement ;
  - étiquettes `v*` immuables (une version publiée ne se réécrit pas) ;
  - environnements `destructive-migration`, `hetzner-destroy`, `k3s-minor` avec relecture humaine ;
  - jeton `GITHUB_TOKEN` en lecture par défaut, approbation des PR de contributeurs externes, analyse de secrets et blocage au push.
- **Fusion automatique** : `gh pr merge --auto` avec un jeton de `clemuscle-platform-ci` limité au dépôt (`contents` et `pull-requests` en écriture). GitHub attend les contrôles requis, et la fusion faite par une App déclenche la CI de `main`. Les PR venues d'un fork ne sont jamais fusionnées ni publiées.
- **Deux identités** : `clemuscle-platform-ci` (clé dans chaque dépôt d'application, droits bornés par les règles) et `clemuscle-platform-bot` (création et configuration des dépôts, clé seulement sur la machine des agents). La garantie « un humain relit `infra` » suppose que les agents utilisent le bot et non le compte de l'administrateur.

## Réglages GitHub recommandés
Ils sont détaillés dans `docs/reglages-github.md` : règles de branche (rulesets) au niveau de l'organisation, contrôles requis, signatures exigées, interdiction des force-push, CODEOWNERS, environnements `production` et `hetzner` avec relecteurs requis pour `tofu apply`, rotation des clés des GitHub Apps tous les 90 jours.
