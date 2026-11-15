# 0016 — Cible Hetzner avec kube-hetzner, nœud unique extensible

**Statut** : proposée · 2026-11-15

## Décision
- Module `kube-hetzner/kube-hetzner/hcloud` **v3.2.1**. L'OS est openSUSE Leap Micro (le choix par défaut du module), avec k3s et embedded etcd.
- Un seul `control_plane_nodepools` avec `count = var.node_count` (1 par défaut, 3 plus tard) et aucun agent. Le type de serveur est une variable : `cx33` (x86, 8 Go) par défaut, `cax21` (ARM, 8 Go) en alternative, puisque les images sont multi-architecture.
- Réglages de kube-hetzner :
  | Variable | Valeur | Raison |
  |---|---|---|
  | `enable_klipper_metal_lb` | automatique sur un nœud | pas de load balancer payant |
  | `ingress_controller` | `traefik` | ADR-0004 |
  | `enable_cert_manager` | `false` | installé par Flux |
  | `enable_system_upgrade_controller` | `false` | installé par Flux (ADR-0013) |
  | Kured (option de désactivation) | `false` | installé par Flux |
  | `automatically_upgrade_kubernetes` | `false` | versions figées dans Git |
  | `node_transport_mode` | `tailscale` | ADR-0012 |
  | `firewall_ssh_source`, `firewall_kube_api_source` | `[]` | fermés en public |
  | `extra_firewall_rules` | 80/443 depuis les IP Cloudflare | ADR-0011 |
  | `etcd_s3_backup` | bucket `-etcd` | ADR-0013 |
  | `enable_secrets_encryption` | `true` | Secrets chiffrés au repos |
  | `create_kubeconfig` | `false` | kubeconfig en sortie sensible, pas sur disque |
- Ensuite, `tofu/envs/hetzner` appelle `modules/flux-bootstrap` sur le cluster créé, via les fournisseurs `kubernetes` et `helm` alimentés par les sorties de kube-hetzner.
- Image OS : kube-hetzner construit un instantané MicroOS/Leap Micro avec Packer, une fois. Cette étape est documentée dans le runbook de migration, et elle crée un instantané facturé de quelques centimes par mois.

## Conséquences
- Avant toute mise en route, il faudra vérifier les prix et la disponibilité des types de serveurs, et relire le README de kube-hetzner à cette version.
- Le passage à 3 nœuds revient à changer une variable. Il faudra alors 2 instances CNPG en production, réévaluer klipper face à un load balancer Hetzner (~6 €/mois), et faire passer Kured et SUC en drain.
