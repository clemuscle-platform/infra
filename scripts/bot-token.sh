#!/usr/bin/env bash
# Affiche un jeton d'installation (1 h) de la GitHub App clemuscle-platform-bot, l'identité des agents.
# Usage : export GH_TOKEN=$(scripts/bot-token.sh)   (ne jamais l'afficher ni le journaliser)
# Fichiers locaux, hors dépôt (droits 600) :
#   ~/.config/platform/platform-bot.env      PLATFORM_BOT_CLIENT_ID=...
#   ~/.config/platform/platform-bot-app.pem  clé privée de l'App
set -euo pipefail

CONF=${PLATFORM_CONFIG_DIR:-$HOME/.config/platform}
ORG=${GITHUB_ORG:-clemuscle-platform}
# shellcheck source=/dev/null
source "$CONF/platform-bot.env"
KEY="$CONF/platform-bot-app.pem"
: "${PLATFORM_BOT_CLIENT_ID:?absent de $CONF/platform-bot.env}"
[[ -r $KEY ]] || { echo "clé introuvable : $KEY" >&2; exit 1; }

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
now=$(date +%s)
header=$(printf '{"alg":"RS256","typ":"JWT"}' | b64url)
payload=$(printf '{"iat":%d,"exp":%d,"iss":"%s"}' $((now - 60)) $((now + 540)) "$PLATFORM_BOT_CLIENT_ID" | b64url)
signature=$(printf '%s.%s' "$header" "$payload" | openssl dgst -sha256 -sign "$KEY" -binary | b64url)
jwt="$header.$payload.$signature"

api() { curl -fsS -H "Authorization: Bearer $jwt" -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" "$@"; }
installation=$(api "https://api.github.com/orgs/$ORG/installation" | jq -r .id)
api -X POST "https://api.github.com/app/installations/$installation/access_tokens" | jq -r .token
