# 0009 — PostgreSQL avec CloudNativePG et le plugin Barman Cloud

**Statut** : proposée · 2026-11-15

## Contexte
CloudNativePG 1.30 marque la sauvegarde intégrée (`spec.backup.barmanObjectStore`) comme dépréciée depuis la 1.26, au profit du **plugin Barman Cloud** (v0.16.0).

## Décision
- L'opérateur CNPG (1.30.x) et le plugin Barman Cloud sont dans `cluster/base/cnpg/`, installés par Flux. cert-manager doit être présent, car le plugin en a besoin.
- Une base par application et par niveau, dans l'espace de noms de l'application. Le module `postgres` du modèle fournit `Cluster` + `ObjectStore` + `ScheduledBackup`.
  | Niveau | Instances | Sauvegarde | Volume |
  |---|---|---|---|
  | prod | 1 sur 1 nœud, 2 sur 3 nœuds (variable) | WAL continu + base quotidienne, rétention 30 j | `${storage_class}` 10Gi |
  | staging | 1 | base quotidienne, rétention 7 j | 5Gi |
  | preview | 1 | aucune ; données de démonstration (seed) | `${preview_storage_class}` 1Gi |
- Les migrations sont exécutées par un Job `migrate`, avec l'annotation Flux qui le place avant le déploiement (`kustomize.toolkit.fluxcd.io/...` + `dependsOn`).
- **Migrations destructives** : la règle est expand/contract. La CI de l'application analyse les migrations nouvelles ou modifiées. Si elle trouve `DROP TABLE`, `DROP COLUMN`, `TRUNCATE`, `ALTER ... RENAME` ou `ALTER COLUMN ... TYPE`, elle pose le contrôle requis `needs-human` à l'échec. Seule l'approbation d'un membre de l'équipe `platform-admins` débloque la PR.
- Restauration : un nouveau `Cluster` avec `bootstrap.recovery` depuis l'`ObjectStore` (runbook dédié).

## Conséquences
- Il n'y a pas de PgBouncer au départ, ce qui économise de la mémoire. On peut ajouter un `Pooler` CNPG par application si besoin.
- La base de production ne peut pas être supprimée par un simple retrait de manifeste. Flux applique le label `kustomize.toolkit.fluxcd.io/prune: disabled`, et la VAP n° 7 la protège aussi.
