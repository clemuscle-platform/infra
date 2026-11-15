#!/usr/bin/env bash
# Génère les secrets de l'environnement LOCAL avec des valeurs aléatoires et les chiffre
# directement avec SOPS (clé publique age de .sops.yaml). Rien n'est écrit en clair sur disque.
# Ne remplace pas un fichier existant, sauf avec --force.
# Ces valeurs ne protègent que le k3s du PC ; les secrets Hetzner sont fournis à part (docs/secrets-de-depart.md).
set -euo pipefail
cd "$(dirname "$0")/.."
FORCE="${1:-}"

hex() { openssl rand -hex "$1"; }

# write_secret <chemin> : lit le YAML en clair sur stdin, écrit le fichier chiffré.
write_secret() {
  local path=$1
  if [[ -e $path && $FORCE != --force ]]; then
    echo "existe déjà : $path (utiliser --force pour régénérer)"; cat >/dev/null; return
  fi
  mkdir -p "$(dirname "$path")"
  sops --encrypt --filename-override "$path" --input-type yaml --output-type yaml /dev/stdin > "$path.tmp"
  mv "$path.tmp" "$path"
  echo "écrit : $path"
}

# Clé d'accès S3 de la plateforme (format Garage : GK + 24 hex / 64 hex).
S3_KEY="GK$(hex 12)"
S3_SECRET="$(hex 32)"
O2_EMAIL="admin@platform.localhost"
O2_PASSWORD="$(hex 16)"
O2_AUTH="Basic $(printf '%s:%s' "$O2_EMAIL" "$O2_PASSWORD" | base64 -w0)"

write_secret cluster/local/controllers/garage/garage-secrets.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: garage-secrets
  namespace: platform-s3
type: Opaque
stringData:
  GARAGE_RPC_SECRET: "$(hex 32)"
  GARAGE_ADMIN_TOKEN: "$(hex 32)"
  AWS_ACCESS_KEY_ID: "$S3_KEY"
  AWS_SECRET_ACCESS_KEY: "$S3_SECRET"
EOF

write_secret cluster/local/configs/secrets/platform-s3.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: platform-s3
  namespace: flux-system
type: Opaque
stringData:
  AWS_ACCESS_KEY_ID: "$S3_KEY"
  AWS_SECRET_ACCESS_KEY: "$S3_SECRET"
  AWS_DEFAULT_REGION: "garage"
EOF

write_secret cluster/local/controllers/secrets/openobserve-root.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: openobserve-root
  namespace: platform-observability
type: Opaque
stringData:
  ZO_ROOT_USER_EMAIL: "$O2_EMAIL"
  ZO_ROOT_USER_PASSWORD: "$O2_PASSWORD"
EOF

write_secret cluster/local/controllers/secrets/openobserve-ingest.sops.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: openobserve-ingest
  namespace: platform-otel
type: Opaque
stringData:
  O2_AUTH: "$O2_AUTH"
EOF
