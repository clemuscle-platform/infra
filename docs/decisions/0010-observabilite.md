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

## slo-sync (révision du 2026-11-21)
- **Exécution** : CronJob toutes les 5 min dans `platform-observability` (là où sont les identifiants d'OpenObserve, qui n'en sortent pas). Lecture seule des ConfigMaps `platform.io/slo=true` (ClusterRole `list configmaps`, aucun accès aux Secrets). Secret `slo-sync` : le seul jeton d'alert-bridge, généré avec les autres secrets.
- **Alertes** : deux par objectif (fenêtres rapide et lente de `alerting`), nommées `slo-<app>-<tier>-<objectif>-<fast|slow>`, dans le dossier OpenObserve `slo`. Requête SQL sur les traces SERVER du namespace `app-<app>-<tier>` : une ligne n'est renvoyée que si la part de requêtes mauvaises dépasse `burn_rate × (1 − target)`, avec au moins 20 requêtes sur la fenêtre. Sévérité : `critical` pour la fenêtre rapide en prod, `warning` sinon. Pas d'alerte en préversion.
- **Notification** : un modèle et une destination uniques (`platform-alert-bridge`) vers `/hooks/generic` ; l'application, le tier et la sévérité passent par les `context_attributes` de l'alerte. Rétablissement notifié (`resolved`), alert-bridge ferme le ticket.
- **Réconciliation** : création, mise à jour si l'empreinte (tag `slo-hash:`) a changé, suppression des alertes plus déclarées. Seules les alertes taguées `managed-by:slo-sync` sont touchées. Un fichier invalide est signalé (code de sortie 1) et les alertes existantes de cette application sont conservées ; si la liste des ConfigMaps échoue, rien n'est appliqué.
- **Sécurité** : le contenu de `slo.yaml` vient des applications. Tout ce qui entre dans le SQL est validé (expressions régulières, routes sans guillemets) et le namespace doit correspondre au service et au tier déclarés : une application ne peut pas créer d'alertes pour une autre. `indicator.good` n'accepte que la forme documentée (le SQL est généré, jamais recopié).
- **Garde-fou SSRF d'OpenObserve** : il refuse par défaut les destinations internes. Exception unique, à correspondance exacte : `ZO_SSRF_ALLOWED_HOSTS=alert-bridge.platform-ops.svc.cluster.local` (code de `ssrf_guard.rs` lu dans la v1.1.2). `ZO_SKIP_SSRF_CHECKS` n'est pas utilisé. Le chart ne redémarre pas OpenObserve quand sa configuration change : `kubectl rollout restart statefulset/openobserve -n platform-observability`.
- **Vérifié sur OpenObserve 1.1.2** (instance locale, 2026-11-21) : noms des champs de traces (`service_service_namespace`, `span_kind = '2'`, `http_response_status_code` en texte, `duration` en microsecondes), requêtes `HAVING` sans `GROUP BY`, substitution de `{alert_name}`, `{alert_status}` et des `context_attributes` dans le modèle, réception par alert-bridge.

## Alternatives écartées
- La pile Prometheus, Loki et Tempo : trois systèmes, et bien plus de 1 Go de RAM.
- Le masquage dans OpenObserve (fonctions VRL) : la donnée transiterait en clair jusqu'au stockage.

## Conséquences
- En local, kube-prometheus-stack (déjà installé à la main) n'est pas touché. Il fera doublon jusqu'à son retrait.
