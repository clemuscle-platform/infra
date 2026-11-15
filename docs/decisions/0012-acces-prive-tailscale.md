# 0012 — Accès privé par Tailscale

**Statut** : proposée · 2026-11-15

## Décision
- Sur Hetzner, seuls les **ports 80 et 443** sont publics (pare-feu Hetzner, réservés aux plages IP de Cloudflare).
- **SSH et l'API Kubernetes** passent uniquement par Tailscale. On utilise `node_transport_mode = "tailscale"` de kube-hetzner avec `firewall_ssh_source = []` et `firewall_kube_api_source = []`. Le kubeconfig vise l'IP ou le nom MagicDNS du nœud sur le tailnet.
- Les **interfaces internes** (OpenObserve, Garage si besoin, interface Flux) sont exposées sur le tailnet par l'**opérateur Tailscale Kubernetes** (`tailscale.com/expose` ou `ingressClassName: tailscale`), seulement dans la surcouche `hetzner`.
- L'accès est contrôlé par les ACL du tailnet : `tag:k8s-operator` et `tag:platform-node`, accessibles au seul groupe `admins`. Les agents qui tournent sur le PC passent par la machine de l'utilisateur.
- **Local** : pas de Tailscale. L'accès se fait par `kubectl port-forward` ou `*.localhost`.

## Conséquences
- Secrets à fournir : clé d'authentification Tailscale réutilisable avec un tag pour le nœud, client OAuth pour l'opérateur.
- Si Tailscale tombe, on a un accès de secours par la console Hetzner (VNC) et une règle de pare-feu temporaire (runbook).
