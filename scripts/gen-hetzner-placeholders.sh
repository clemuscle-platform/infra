#!/usr/bin/env bash
# Crée les Secrets de l'environnement HETZNER avec des valeurs factices « A-REMPLACER », chiffrées SOPS.
# Les vraies valeurs sont listées dans docs/secrets-de-depart.md et saisies avec « sops <fichier> »
# une fois la clé age hetzner générée (et .sops.yaml mis à jour, puis « sops updatekeys »).
# Ne remplace pas un fichier existant, sauf avec --force.
set -euo pipefail
cd "$(dirname "$0")/.."
P="A-REMPLACER"
FORCE="${1:-}"

write_secret() {
  local path=$1
  if [[ -e $path && $FORCE != --force ]]; then echo "existe déjà : $path"; cat >/dev/null; return; fi
  mkdir -p "$(dirname "$path")"
  sops --encrypt --filename-override "$path" --input-type yaml --output-type yaml /dev/stdin > "$path.tmp"
  mv "$path.tmp" "$path"
  echo "écrit : $path"
}

C=cluster/hetzner/controllers/secrets
G=cluster/hetzner/configs/secrets

write_secret $C/openobserve-root.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: openobserve-root, namespace: platform-observability}
type: Opaque
stringData:
  ZO_ROOT_USER_EMAIL: "$P"
  ZO_ROOT_USER_PASSWORD: "$P"
EOF

write_secret $C/openobserve-ingest.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: openobserve-ingest, namespace: platform-otel}
type: Opaque
stringData:
  O2_AUTH: "Basic $P"
EOF

write_secret $C/openobserve-s3.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: openobserve-s3, namespace: flux-system}
type: Opaque
stringData:
  ZO_S3_ACCESS_KEY: "$P"
  ZO_S3_SECRET_KEY: "$P"
EOF

write_secret $C/cloudflare-api-token.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: cloudflare-api-token, namespace: external-dns}
type: Opaque
stringData:
  api-token: "$P"
EOF

write_secret $C/tailscale-operator-oauth.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: operator-oauth, namespace: tailscale}
type: Opaque
stringData:
  client_id: "$P"
  client_secret: "$P"
EOF

write_secret $G/platform-s3.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: platform-s3, namespace: flux-system}
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: "$P"
  AWS_SECRET_ACCESS_KEY: "$P"
  AWS_DEFAULT_REGION: "fsn1"
EOF

write_secret $G/cloudflare-api-token-cert-manager.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: cloudflare-api-token, namespace: cert-manager}
type: Opaque
stringData:
  api-token: "$P"
EOF

write_secret $G/offsite-credentials.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata: {name: offsite-credentials, namespace: platform-backup}
type: Opaque
stringData:
  RCLONE_CONFIG_SRC_ACCESS_KEY_ID: "$P"
  RCLONE_CONFIG_SRC_SECRET_ACCESS_KEY: "$P"
  RCLONE_CONFIG_DST_ACCESS_KEY_ID: "$P"
  RCLONE_CONFIG_DST_SECRET_ACCESS_KEY: "$P"
EOF
