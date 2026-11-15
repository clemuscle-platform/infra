# 0008 — Stockage objet et sauvegardes

**Statut** : proposée · 2026-11-15

## Contexte
Plusieurs composants ont besoin d'un S3 : l'archivage des WAL CNPG, les instantanés etcd, OpenObserve et les applications qui stockent des fichiers. Les sauvegardes doivent aussi exister hors de l'hébergeur.

## Décision
- **Local** : **Garage** (léger, mono-binaire, compatible S3), déployé par Flux dans `platform-s3`, avec un seul nœud et le stockage en local-path. Les buckets et les clés sont créés par un Job idempotent. MinIO est écarté : rien n'a été publié dans la version communautaire depuis octobre 2025.
- **Hetzner** : **Hetzner Object Storage** est le bucket principal, dans la même région que le serveur. OpenTofu le crée avec le fournisseur `aws`, configuré sur l'endpoint Hetzner. Les clés d'accès se créent à la main dans la console. Buckets prévus :
  - `<prefix>-pg` pour les WAL et les sauvegardes CNPG, versionné, avec cycle de vie à 30 jours ;
  - `<prefix>-etcd` pour les instantanés k3s ;
  - `<prefix>-o2` pour les données OpenObserve ;
  - `<prefix>-apps` pour les données des applications, avec un préfixe par application ;
  - `<prefix>-tfstate` pour l'état OpenTofu, versionné. Il est créé par l'amorçage et n'est jamais détruit par OpenTofu.
- **Hors hébergeur** : un CronJob **rclone** copie chaque jour `-pg`, `-etcd` et `-apps` vers **Cloudflare R2** (ou Backblaze B2, voir l'ADR-0017). La destination a le versionnage activé et une clé d'API en écriture seule quand le fournisseur le permet. Une alerte se déclenche si la dernière copie date de plus de 36 h.
- Un test de restauration mensuel est automatisé : un Job crée un cluster CNPG temporaire depuis la sauvegarde, lance une requête de contrôle, puis le supprime. Le résultat est envoyé à OpenObserve.

## Conséquences
- Les applications reçoivent des identifiants S3 par Secret (`${s3_endpoint}`, bucket, préfixe) : le code ne change pas d'un environnement à l'autre.
- En local, les « sauvegardes » restent sur la même machine. Elles servent à tester les procédures, pas à protéger des données.
