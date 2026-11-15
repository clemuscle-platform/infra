# 0001 — GitOps avec Flux Operator, amorcé par OpenTofu

**Statut** : proposée · 2026-11-15

## Contexte
Flux et Flux Operator sont imposés. Il reste à décider comment Flux est installé et mis à jour, et comment il s'intègre au principe « OpenTofu installe le serveur, k3s et Flux ; Flux installe le reste ».

## Décision
- Flux est installé par **Flux Operator** (chart `oci://ghcr.io/controlplaneio-fluxcd/charts/flux-operator`, 0.62.1), à partir d'une ressource `FluxInstance` qui fige la distribution (Flux 2.10.x).
- L'amorçage passe par le **module OpenTofu officiel `controlplaneio-fluxcd/flux-operator-bootstrap/kubernetes`** (v0.10.0). Un module commun, `tofu/modules/flux-bootstrap`, est appelé par `tofu/envs/local` et par `tofu/envs/hetzner`. Il crée :
  - le Secret `sops-age` (la clé privée est lue depuis un fichier local, jamais depuis Git) ;
  - le Secret de la GitHub App en lecture ;
  - la `FluxInstance`, qui synchronise `cluster/<env>` depuis le dépôt `infra`.
- Après l'amorçage, la `FluxInstance` est **elle-même gérée par Flux** depuis `cluster/base/flux-system/`. Flux se met donc à jour par Git, et Renovate propose les nouvelles versions.
- Composants Flux activés : source, kustomize, helm, notification. Les contrôleurs image-reflector et image-automation ne sont pas installés : la livraison passe par des artefacts OCI (ADR-0002).
- Les préversions de PR et les niveaux de déploiement utilisent **ResourceSet / ResourceSetInputProvider** (types `GitHubPullRequest` et `OCIArtifactTag`).

## Alternatives écartées
- `flux bootstrap github` : il écrit dans le dépôt et utilise une clé de déploiement. Ses manifestes générés sont moins lisibles pour des agents, et il n'apporte ni ResourceSet ni MCP.
- Installation de Flux par Helm depuis OpenTofu, sans l'opérateur : on perdrait les mises à jour déclaratives et les ResourceSet.

## Conséquences
- Avant le premier `tofu apply`, le dépôt `infra` doit exister sur GitHub, avec la GitHub App installée.
- La version de Kubernetes doit rester dans la fenêtre que Flux prend en charge (les trois dernières versions mineures, soit ≥ 1.35.2 pour Flux 2.10). La montée de k3s fait partie de l'entretien.
- Le serveur MCP `flux-operator-mcp` (même version que l'opérateur) est l'interface de lecture des agents (ADR-0014).
