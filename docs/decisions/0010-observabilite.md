# 0010 — OpenTelemetry → OpenObserve, masquage dans le collecteur

**Statut** : proposée · 2026-11-15

## Décision
- **OpenTelemetry Collector** (distribution `otelcol-contrib` 0.165), déployé par le chart officiel en DaemonSet, dans **`platform-otel`** (PSA `privileged`, car il lit `/var/log/pods` par hostPath). OpenObserve reste dans `platform-observability` (PSA `restricted`). Récepteurs :
  - `otlp` (gRPC et HTTP) pour les applications ;
  - `filelog` pour les journaux des conteneurs ;
  - `kubeletstats` pour les métriques des pods et des nœuds ;
  - `k8s_cluster`, `k8sobjects` (événements) et `prometheus` (cibles annotées).
  Il n'y a pas d'opérateur OTel : on évite un webhook et de la RAM.
- **Masquage avant export**, dans cet ordre :
  1. `transform` (OTTL) : retire les attributs `http.request.header.authorization`, `cookie`, `set-cookie`, `x-api-key`, et remplace les valeurs des paramètres de requête `token`, `password`, `secret`, `code` ;
  2. `redaction` (`allow_all_keys: true`, `blocked_values`) : jetons GitHub (`gh[pousr]_…`, `github_pat_…`), clés AWS/S3 (`AKIA…`), JWT, blocs `-----BEGIN … PRIVATE KEY-----`, chaînes de connexion `postgres://user:pass@`, adresses e-mail, numéros de carte (Luhn approximé), IBAN, numéros de téléphone français ;
  3. les mêmes expressions régulières sont appliquées au corps des journaux (`replace_pattern` sur `body`).
  Les règles sont testées dans la CI : un collecteur en mode `file` traite `tests/redaction/input.jsonl` et la sortie est comparée à `expected.jsonl`.
- **OpenObserve** v1.1.x, en mode nœud unique (chart `openobserve-standalone`), dans `platform-observability`. Stockage sur disque local en local, sur le bucket S3 `-o2` sur Hetzner. Rétention de 30 jours. Les identifiants d'ingestion sont dans un Secret SOPS.
- Les SDK OTel des applications sont configurés par des variables d'environnement standard (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=<niveau>`).
- **Objectifs de service** : `deploy/slo.yaml` définit, pour chaque application, le taux d'erreur 5xx et le temps de réponse p95, sur 5 min (alerte rapide) et 1 h (alerte lente). Un Job de la plateforme (`slo-sync`) traduit ces fichiers en alertes OpenObserve via son API.

## Alternatives écartées
- La pile Prometheus, Loki et Tempo : trois systèmes, et bien plus de 1 Go de RAM.
- Le masquage dans OpenObserve (fonctions VRL) : la donnée transiterait en clair jusqu'au stockage.

## Conséquences
- En local, kube-prometheus-stack (déjà installé à la main) n'est pas touché. Il fera doublon jusqu'à son retrait.
