#!/usr/bin/env bash
# Teste le masquage des secrets et données personnelles du collecteur OTel (ADR-0010), avec le VRAI
# binaire du collecteur et les processeurs EXTRAITS de otel-values.yaml (aucune copie à maintenir).
#   filelog (input.jsonl) → transform/redact → redaction → file (sortie JSON) → vérification Python.
# Prérequis : docker. Image : celle de l'appVersion du chart (variable OTEL_IMAGE pour la changer).
set -euo pipefail
cd "$(dirname "$0")/.."
T=cluster/base/controllers/observability/tests/redaction
WORK=$(mktemp -d -p "${TMPDIR:-/tmp}")
chmod 0777 "$WORK"
OTEL_IMAGE="${OTEL_IMAGE:-otel/opentelemetry-collector-contrib:0.163.2}"

python3 -I - cluster/base/controllers/observability/otel-values.yaml "$WORK/config.yaml" <<'PY'
import sys, yaml
values = yaml.safe_load(open(sys.argv[1]))
procs = values["config"]["processors"]
cfg = {
    "receivers": {"filelog": {
        "include": ["/work/input.jsonl"], "start_at": "beginning",
        "operators": [
            {"type": "json_parser", "parse_to": "attributes"},
            {"type": "move", "from": "attributes.msg", "to": "body"},
        ],
    }},
    "processors": {"transform/redact": procs["transform/redact"], "redaction": procs["redaction"]},
    "exporters": {"file": {"path": "/work/out.json"}},
    "service": {"pipelines": {"logs": {
        "receivers": ["filelog"], "processors": ["transform/redact", "redaction"], "exporters": ["file"]}}},
}
yaml.safe_dump(cfg, open(sys.argv[2], "w"), sort_keys=False)
PY
cp "$T/input.jsonl" "$WORK/"
chmod 0644 "$WORK"/*

echo "== otelcol validate ($OTEL_IMAGE)"
docker run --rm -v "$WORK:/work" "$OTEL_IMAGE" validate --config=/work/config.yaml
echo "== traitement des journaux de test"
docker run --rm -v "$WORK:/work" --entrypoint /otelcol-contrib "$OTEL_IMAGE" --config=/work/config.yaml \
  > "$WORK/collector.log" 2>&1 &
pid=$!
for _ in $(seq 1 30); do
  [[ -s $WORK/out.json ]] && [[ $(grep -o '"case"' "$WORK/out.json" | wc -l) -ge $(wc -l < "$T/input.jsonl") ]] && break
  sleep 1
done
sleep 2
docker ps -q --filter ancestor="$OTEL_IMAGE" | xargs -r docker stop >/dev/null
wait $pid 2>/dev/null || true
python3 -I "$T/check.py" "$WORK/out.json" "$T/input.jsonl" || { tail -20 "$WORK/collector.log"; exit 1; }
