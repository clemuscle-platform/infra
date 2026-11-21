# Runbook : restaurer une base PostgreSQL (CloudNativePG + Barman Cloud)

Les bases des applications (module `postgres-backup`, staging et prod) archivent leurs WAL en continu
et font une sauvegarde complète chaque jour vers `s3://<bucket-pg>/<namespace>/` (ADR-0009).
Une restauration crée **toujours un nouveau cluster** à partir du stockage objet : la base d'origine
n'est jamais modifiée.

## 1. Test de restauration (sans risque, à faire régulièrement)
Vérifié le 2026-11-21 sur le k3s local (`hello`, staging) : 3 lignes témoins écrites par l'API,
sauvegarde à la demande, restauration dans un nouveau cluster, données identiques.

1. Sauvegarde à la demande (facultatif : la sauvegarde quotidienne et les WAL suffisent) :
   ```yaml
   apiVersion: postgresql.cnpg.io/v1
   kind: Backup
   metadata: {name: <app>-test-restauration, namespace: app-<app>-<tier>}
   spec:
     cluster: {name: <app>-pg}
     method: plugin
     pluginConfiguration: {name: barman-cloud.cloudnative-pg.io}
   ```
   Attendre `kubectl get backup <nom> -n <ns>` → `completed`.
2. Cluster restauré, dans le même namespace :
   ```yaml
   apiVersion: postgresql.cnpg.io/v1
   kind: Cluster
   metadata:
     name: <app>-pg-restore
     namespace: app-<app>-<tier>
     labels: {app.kubernetes.io/name: <app>, platform.io/app: <app>}
   spec:
     instances: 1
     enableSuperuserAccess: false
     bootstrap:
       recovery: {source: origine, database: app, owner: app}
     externalClusters:
       - name: origine
         plugin:
           name: barman-cloud.cloudnative-pg.io
           parameters: {barmanObjectName: <app>-backup, serverName: <app>-pg}
     storage: {size: 1Gi, storageClass: <classe du niveau>}
     resources:
       requests: {cpu: 50m, memory: 192Mi}
       limits: {memory: 384Mi}
   ```
   Attendre `Cluster in healthy state` (environ 1 min pour une petite base).
3. Comparer les données (lecture seule, connexion locale dans le pod) :
   ```bash
   kubectl exec -n <ns> <app>-pg-restore-1 -c postgres -- psql -d app -At -c "SELECT count(*) FROM <table>"
   kubectl exec -n <ns> <app>-pg-1        -c postgres -- psql -d app -At -c "SELECT count(*) FROM <table>"
   ```
4. Supprimer le cluster de test : `kubectl delete cluster <app>-pg-restore -n <ns>`.

Les quotas des namespaces prévoient la place d'un second PostgreSQL pour ce test
(`cluster/base/tenants/app-tenants.yaml`).

## 2. Restauration réelle (incident : données perdues ou corrompues)
**Décision humaine obligatoire en production** : la VAP `platform-protect-prod-data` bloque la suppression
de la base, et un agent ne remplace jamais une base de production seul.

1. Restaurer dans un nouveau cluster comme au §1. Pour revenir à un instant précis (avant l'erreur),
   ajouter `recoveryTarget: {targetTime: "2026-11-21T18:25:00Z"}` sous `bootstrap.recovery`.
2. Vérifier les données restaurées.
3. Basculer l'application : faire pointer `DATABASE_URL` vers le Secret `<app>-pg-restore-app`
   (PR sur le dépôt de l'application), ou renommer selon la procédure CNPG.
4. Une fois la bascule validée, et seulement alors, retirer l'ancien cluster.

**Non encore vérifié** : `recoveryTarget` (retour à un instant précis) et la bascule de l'étape 3.
À tester sur staging avant d'en avoir besoin en production.

## Dépannage
- `exceeded quota: platform-quota` : vérifier `kubectl describe resourcequota platform-quota -n <ns>`.
  Le pod PostgreSQL compte avec le sidecar Barman (environ 640Mi de limite au total).
- `Couldn't find target pod` sur un `Backup` : le cluster n'a pas encore de primaire, la sauvegarde réessaie seule.
- Erreurs d'accès S3 : Secret `platform-s3` du namespace (copié par la plateforme) et `ObjectStore <app>-backup`.
