# 0004 — Réutiliser les composants de k3s ; Ingress plutôt que Gateway API

**Statut** : proposée · 2026-11-15

## Contexte
k3s fournit Traefik (3.6, CRD Gateway API présentes), servicelb (klipper), local-path et metrics-server. kube-hetzner installe aussi Traefik, avec des réglages adaptés à Hetzner, et passe en klipper sur un seul nœud (pas de load balancer payant).

## Décision
- **Traefik** est fourni par la distribution : k3s en local, kube-hetzner sur Hetzner. La base suppose seulement l'existence d'une `IngressClass` nommée `traefik`. Les réglages se font par `HelmChartConfig` dans les surcouches (redirection HTTP→HTTPS, en-têtes de sécurité, en-têtes du proxy Cloudflare pour l'IP réelle).
- **servicelb/klipper** est utilisé en local et sur Hetzner tant qu'il n'y a qu'un nœud.
- Stockage : **local-path** en local et **hcloud-volumes** (CSI Hetzner) sur Hetzner. Les manifestes ne nomment pas de StorageClass : ils prennent la classe par défaut ou `${storage_class}`.
- **metrics-server** est conservé, pour `kubectl top` et les HPA éventuels.
- **Ingress** (`networking.k8s.io/v1`) plutôt que Gateway API pour l'instant : c'est pris en charge sans réglage par Traefik dans k3s, cert-manager et external-dns. La migration vers Gateway API fera l'objet d'une fiche séparée.

## Alternatives écartées
- ingress-nginx : son développement s'est arrêté en 2026.
- Gateway API tout de suite : il faudrait activer le fournisseur Gateway de Traefik dans les deux distributions, sans bénéfice immédiat.
- Longhorn : trop gourmand en mémoire pour 8 Go et redondant sur un seul nœud.

## Conséquences
- Les surcouches contiennent un `HelmChartConfig` pour Traefik.
- Les volumes `hcloud-volumes` font au moins 10 Go chacun, ce qui coûte un peu par PVC. Les bases de staging et de préversion utilisent donc de petits volumes, et les préversions peuvent utiliser local-path (variable `${preview_storage_class}`).
