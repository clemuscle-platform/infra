# 0002 — Livraison des applications par artefacts OCI signés, trois niveaux

**Statut** : proposée · 2026-11-15

## Contexte
Chaque application a un dossier `deploy/` décrivant trois niveaux : préversion par PR, staging et production. Il faut que Flux ne déploie jamais une image qui n'existe pas encore, que les agents n'écrivent que par Git, et que la promotion soit traçable.

## Décision
- La CI de l'application (workflow réutilisable du dépôt `infra`) suit cet ordre :
  1. tests et scans ;
  2. construction et publication de l'image multi-architecture `ghcr.io/<org>/<app>:<sha>` ;
  3. remplacement de l'image dans `deploy/` par son **digest** ;
  4. `flux push artifact` de `deploy/` vers `oci://ghcr.io/<org>/<app>-deploy` ;
  5. signature de l'artefact et de l'image avec **cosign en mode keyless** (OIDC GitHub).
- Tags de l'artefact selon l'événement :
  | Événement | Tag de l'artefact | Niveau | Espace de noms |
  |---|---|---|---|
  | PR avec le label `preview` | `pr-<n>` | préversion | `app-<app>-pr-<n>` |
  | push sur `main` | `main` | staging | `app-<app>-staging` |
  | étiquette `vX.Y.Z` | `X.Y.Z` | production | `app-<app>-prod` |
- Côté cluster, `cluster/apps/<app>.yaml` contient deux `ResourceSetInputProvider` : la déclaration (`Static`) et les préversions (`GitHubPullRequest`). Le ResourceSet commun `app-tenants` crée, pour chaque niveau, une `OCIRepository` (avec `verify.provider: cosign` et l'identité du workflow réutilisable `infra/.github/workflows/app-ci.yml`) et une `Kustomization` qui applique `deploy/overlays/<niveau>` (`preview`, `staging`, `prod`). La production suit `semver: ">=0.1.0"`.
- Les préversions sont générées par un `ResourceSetInputProvider` de type `GitHubPullRequest`, filtré sur le label `preview`, qui crée et supprime les espaces de noms `pr-<n>`. Le nombre de préversions simultanées est plafonné (`filter.limit`).
- Le statut du déploiement est renvoyé sur le commit GitHub par un `Provider` de notification `github`. L'URL de la préversion est commentée sur la PR (`githubpullrequestcomment`).

## Alternatives écartées
- **Flux image-automation**, où Flux écrit le tag dans Git : Flux deviendrait un auteur de commits, avec deux contrôleurs de plus (RAM) et des commits parasites.
- **GitRepository sur le dépôt de l'application** : un manifeste pourrait être déployé avant que son image n'existe, sans signature.
- **La CI de l'application écrit le tag dans le dépôt `infra`** : il faudrait un jeton qui écrit dans `infra`, ce qui donne trop de droits.

## Conséquences
- GHCR est privé : Flux lit les artefacts avec la GitHub App (fournisseur `github` sur `OCIRepository`, ou Secret `dockerconfigjson` renouvelé).
- Un retour arrière en production consiste à publier une nouvelle étiquette depuis un commit antérieur. On peut aussi figer temporairement `semver` dans `cluster/apps/<app>.yaml`, mais cette modification-là demande une validation humaine.
- Pour tester en local sans GitHub, `make deploy ENV=local` dans un dépôt d'application publie l'artefact dans GHCR sous un tag `dev-<user>`.
