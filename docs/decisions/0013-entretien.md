# 0013 — Entretien : Renovate, System Upgrade Controller, Kured, etcd

**Statut** : proposée · 2026-11-15

## Décision
- **Renovate**, auto-hébergé dans GitHub Actions (`renovatebot/github-action`, programmé tous les jours à 5 h), avec une GitHub App dédiée. Il couvre les charts Helm (HelmRelease), les images, les GitHub Actions (par digest), les versions OpenTofu, les modules et fournisseurs, les outils de `tools.versions`, et `FluxInstance.spec.distribution.version`.
  - Correctifs et versions mineures : fusion automatique si les contrôles passent, sauf les règles de la ligne suivante.
  - Versions majeures, CNPG, k3s, Flux : PR seulement, avec une étiquette `needs-human` pour les versions mineures de k3s.
- **System Upgrade Controller** (0.21), seulement sur Hetzner, installé par Flux, avec deux `Plan` dans `cluster/hetzner/upgrades/` :
  - `k3s-server` avec une `version` figée (pas de `channel`), pour que chaque montée de version soit un commit relu ;
  - fenêtre de maintenance `window` le mardi de 3 h à 5 h (Europe/Paris), en cordon sans drain sur un seul nœud.
  Les correctifs (1.35.x) sont proposés par Renovate et fusionnés automatiquement. Une version mineure demande une validation humaine (CODEOWNERS + contrôle `k3s-minor-guard`).
- **Kured** (1.23), seulement sur Hetzner : redémarrage quand `/var/run/reboot-required` existe, ou selon le marqueur de Leap Micro, à vérifier en phase 3. Fenêtre du mardi au jeudi de 3 h à 5 h. Un seul nœud à la fois, avec des sondes d'alertes bloquantes.
- Mises à jour de l'OS : mises à jour transactionnelles de Leap Micro (timer de kube-hetzner, `automatically_upgrade_os`). La documentation de kube-hetzner les déconseille hors HA. On les garde actives ici, parce que Kured encadre le redémarrage et qu'un nœud unique tolère une courte coupure : c'est un compromis à valider.
- **Instantanés etcd** : `etcd_s3_backup` de kube-hetzner, toutes les 6 h, 28 conservés, vers `<prefix>-etcd`. Il n'y a pas d'etcd en local (SQLite).

## Conséquences
- kube-hetzner est configuré avec `enable_system_upgrade_controller = false` et Kured désactivé, puisque Flux s'en charge (ADR-0016).
