# 0011 — DNS et TLS : Cloudflare, external-dns, cert-manager

**Statut** : proposée · 2026-11-15

## Décision
- **Hetzner** :
  - Cloudflare fait autorité sur la zone `${cluster_domain}` (provisoirement `example.com`). Les enregistrements des applications sont en mode `proxied` (CDN et anti-DDoS), avec le mode SSL **Full (strict)**.
  - **external-dns** (0.24), fournisseur `cloudflare`, avec `--domain-filter=${cluster_domain}`, `--policy=upsert-only` en production pour qu'il ne supprime jamais rien seul, `--txt-owner-id=<cluster>`, et l'annotation `external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"`.
  - **cert-manager** (1.21) : `ClusterIssuer` `letsencrypt` (production et staging) en DNS-01 Cloudflare. Cela marche derrière le proxy et permet un certificat générique pour les préversions (`*.preview.${cluster_domain}`).
  - Jeton API Cloudflare limité à la zone, avec les droits `Zone:DNS:Edit` et `Zone:Zone:Read`.
  - Le pare-feu Hetzner n'ouvre les ports 80 et 443 qu'aux plages IP de Cloudflare (liste dans une variable OpenTofu, mise à jour par Renovate ou une tâche programmée).
- **Local** : `ClusterIssuer` `selfsigned-ca`, une CA locale générée par cert-manager. Il n'y a pas d'external-dns, puisque `*.localhost` se résout tout seul vers 127.0.0.1. Il faut faire confiance à la CA à la main dans le navigateur, si on le souhaite.
- La base utilise toujours `cert-manager.io/cluster-issuer: ${cluster_issuer}` sur les Ingress.

## Conséquences
- Il faut un domaine réel avant la migration vers Hetzner.
- En local, la release cert-manager existante est reprise par la HelmRelease Flux, avec le même nom (`cert-manager`) et le même espace de noms (voir le runbook d'installation locale).
