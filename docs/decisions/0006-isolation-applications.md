# 0006 — Isolation : un espace de noms par application et par niveau

**Statut** : proposée · 2026-11-15

## Décision
- Les espaces de noms sont `app-<nom>-prod`, `app-<nom>-staging` et `app-<nom>-pr-<n>`. Chacun porte les labels `platform.io/managed=true`, `platform.io/app=<nom>`, `platform.io/tier=<prod|staging|preview>` et `pod-security.kubernetes.io/enforce=restricted`.
- Un composant Kustomize, `cluster/base/tenants/component`, génère pour chaque espace de noms :
  - `NetworkPolicy default-deny` (entrée et sortie) ;
  - les autorisations : DNS vers kube-dns, entrée depuis l'espace de noms de Traefik, sortie vers l'OTel Collector, et trafic interne à l'espace de noms (application ↔ sa base) ;
  - la sortie vers Internet, seulement si l'application la déclare (`egress: [smtp, https]` dans `cluster/apps/<app>.yaml`). Elle exclut toujours les plages privées et l'IP de métadonnées `169.254.169.254` ;
  - `ResourceQuota` et `LimitRange` selon le niveau ;
  - un `ServiceAccount` `flux-deployer` et un `RoleBinding` vers `admin` sur cet espace de noms uniquement. Les Kustomizations de l'application tournent sous ce compte.
- Quotas par défaut, modifiables par application :
  | Niveau | requests.memory | limits.memory | requests.cpu | PVC |
  |---|---|---|---|---|
  | prod | 768Mi | 1Gi | 500m | 3 |
  | staging | 384Mi | 512Mi | 250m | 2 |
  | preview | 256Mi | 384Mi | 200m | 1 |

## Alternatives écartées
- Un espace de noms par application, partagé entre les niveaux : on ne pourrait pas isoler les préversions, et les quotas seraient mélangés.
- Kyverno `generate` : voir l'ADR-0005.

## Conséquences
- Le contrôleur de NetworkPolicy intégré à k3s (kube-router) suffit, sans Cilium.
- Une application ne peut ni joindre une autre application, ni la base d'une autre application.
