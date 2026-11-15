# 0011 — DNS et TLS : Cloudflare, external-dns, cert-manager

**Statut** : proposée · 2026-11-15

## Décision
- **Hetzner** :
  - Cloudflare fait autorité sur la zone `${cluster_domain}` (provisoirement `example.com`). Les enregistrements des applications sont en mode `proxied` (CDN et anti-DDoS), avec le mode SSL **Full (strict)**.
  - **external-dns** (0.24), fournisseur `cloudflare`, avec `--domain-filter=${cluster_domain}`, `--policy=upsert-only` en production pour qu'il ne supprime jamais rien seul, `--txt-owner-id=<cluster>`, et l'annotation `external-dns.alpha.kubernetes.io/cloudflare-proxied: "true"`.
  - **cert-manager** (1.21) : `ClusterIssuer` `letsencrypt` (production et staging) en DNS-01 Cloudflare. Cela marche derrière le proxy. Les hôtes restent sur **un seul niveau** de sous-domaine (`<app>.<domaine>`, `<app>-staging.<domaine>`, `<app>-pr-<n>.<domaine>`), parce que le certificat universel gratuit de Cloudflare ne couvre que `*.<domaine>`.
  - Jeton API Cloudflare limité à la zone, avec les droits `Zone:DNS:Edit` et `Zone:Zone:Read`.
  - Le pare-feu Hetzner de kube-hetzner ouvre **toujours** 80 et 443 à tout Internet, sans option pour le restreindre (vérifié dans le code du module v3.2.1, phase 3). L'exigence « seuls 80/443 publics » est respectée. Traefik ne fait confiance à `X-Forwarded-For` que pour les plages IP de Cloudflare, lues par OpenTofu sur cloudflare.com/ips-v4 et ips-v6. **Durcissement à décider** : refuser le trafic qui ne vient pas de Cloudflare, soit par un middleware Traefik `ipAllowList` au niveau des entrypoints, soit par *Authenticated Origin Pulls* (mTLS Cloudflare → Traefik).
- **Local** : `ClusterIssuer` `selfsigned-ca`, une CA locale générée par cert-manager. Il n'y a pas d'external-dns, puisque `*.localhost` se résout tout seul vers 127.0.0.1. Il faut faire confiance à la CA à la main dans le navigateur, si on le souhaite.
- La base utilise toujours `cert-manager.io/cluster-issuer: ${CLUSTER_ISSUER}` sur les Ingress.

## Conséquences
- Il faut un domaine réel avant la migration vers Hetzner.
- En local, la release cert-manager existante est reprise par la HelmRelease Flux, avec le même nom (`cert-manager`) et le même espace de noms (voir le runbook d'installation locale).
