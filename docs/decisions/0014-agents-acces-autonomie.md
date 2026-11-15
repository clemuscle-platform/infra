# 0014 — Agents : lecture seule, écriture par Git, règles d'autonomie, alertes → tickets

**Statut** : proposée · 2026-11-15

## Accès en lecture
- Le ServiceAccount `platform-agents/agent-reader` est lié au ClusterRole `agent-reader`. Ses règles sont explicites (pas de `*`) :
  - `get`, `list` et `watch` sur les ressources des groupes de base (sans `secrets`), `apps`, `batch`, `networking.k8s.io`, `policy`, `autoscaling`, `storage.k8s.io`, `apiextensions.k8s.io`, `admissionregistration.k8s.io`, `events.k8s.io` et `metrics.k8s.io` ;
  - les mêmes verbes sur les CRD de la plateforme : `*.toolkit.fluxcd.io`, `fluxcd.controlplane.io`, `cert-manager.io`, `postgresql.cnpg.io`, `barmancloud.cnpg.io` et `upgrade.cattle.io` ;
  - `pods/log` en `get`.
  Il n'a **aucun droit** sur `secrets`, `pods/exec`, `pods/attach`, `pods/portforward`, `serviceaccounts/token`, ni sur `impersonate`.
- Un test automatisé vérifie le rôle : `kubectl auth can-i --as=system:serviceaccount:platform-agents:agent-reader` sur une matrice de cas, lancé par `make test-rbac`, en lecture seule.
- Kubeconfig d'agent : jeton de 24 h, obtenu par `kubectl create token agent-reader --duration=24h` dans `make agent-kubeconfig`.
- Serveur MCP : `flux-operator-mcp serve --read-only --kubeconfig ~/.kube/agent-reader.yaml`, déclaré dans `.mcp.json` à la racine des dépôts.

## Écriture par Git
- Une GitHub App `platform-agents` a les droits : Contents RW, Pull requests RW, Issues RW, Checks R, Metadata R, sans administration ni workflows. Elle est installée sur l'organisation.
- Une GitHub App distincte, `platform-flux`, a les droits Contents R et Packages R. Flux l'utilise.

## Règles d'autonomie (appliquées par la CI et la protection des branches)
| Situation | Comportement |
|---|---|
| PR d'application, tous les contrôles verts, pas de `needs-human` | auto-merge (squash) |
| Plan OpenTofu avec `delete` ou `replace` | contrôle `tofu-destroy-guard` en échec ; il faut l'approbation de `@clemuscle-platform/platform-admins` |
| Migration SQL destructive | contrôle `migration-guard` en échec ; même approbation |
| Changement de version mineure dans `cluster/hetzner/upgrades/` | contrôle `k3s-minor-guard` en échec ; même approbation |
| PR sur `infra` | approbation humaine par défaut (CODEOWNERS), sauf PR Renovate de correctif avec contrôles verts |

## Alertes → tickets (alert-bridge)
- Un petit service maison (Go, image distroless, environ 15 Mo de RAM) tourne dans `platform-ops`. Il reçoit les webhooks :
  - d'OpenObserve (alertes SLO, journaux d'erreur, échec de sauvegarde) ;
  - de Flux (`Provider` de type `generic-hmac` pour les échecs de réconciliation) ;
  - d'UptimeRobot (exposé publiquement sur `hooks.${cluster_domain}/uptimerobot`, avec un secret dans l'en-tête).
- Il calcule une empreinte (`source + alerte + application`), cherche un ticket ouvert qui porte le label `fp:<empreinte>`, puis le commente ou crée un ticket dans le dépôt de l'application (ou dans `infra`). Labels posés : `incident`, `sev:<n>`, `app:<nom>`. Une alerte résolue fait l'objet d'un commentaire, et le ticket est fermé après 1 h sans récidive.
- Il s'authentifie par la GitHub App `platform-alerts` (droit Issues RW uniquement) et génère un jeton d'installation à chaque appel.
- En local, il est en mode `dry-run` : il écrit les tickets dans ses journaux, sans appel à GitHub.

## Conséquences
- Les agents prennent les tickets `incident` en priorité. La procédure est dans `CLAUDE.md` et dans `runbooks/boucles-exploitation.md`.
