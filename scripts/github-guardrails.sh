#!/usr/bin/env bash
# Applique les garde-fous GitHub d'un dépôt de l'organisation (idempotent, ADR-0015) :
# réglages de fusion, Actions, analyse de secrets, règles de protection (github/rulesets/),
# environnements protégés par une relecture humaine.
# Usage : scripts/github-guardrails.sh infra|app <dépôt>
# Jeton : celui de gh (GH_TOKEN), avec le droit Administration sur le dépôt
#   (App clemuscle-platform-bot, ou un administrateur de l'organisation).
set -euo pipefail

KIND=${1:?usage: $0 infra|app <dépôt>}
REPO=${2:?usage: $0 infra|app <dépôt>}
ORG=${GITHUB_ORG:-clemuscle-platform}
ADMIN_LOGIN=${PLATFORM_ADMIN_LOGIN:-clemuscle}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
R="repos/$ORG/$REPO"

case "$KIND" in
  infra) RULESETS=(infra-main release-tags); ENVS=(hetzner-destroy k3s-minor) ;;
  app)   RULESETS=(app-main release-tags);   ENVS=(destructive-migration) ;;
  *) echo "type inconnu : $KIND (infra|app)" >&2; exit 2 ;;
esac

[[ $(gh api "$R" --jq .visibility) == public ]] || {
  echo "$ORG/$REPO n'est pas public : le plan Free n'y applique ni règles ni environnements protégés" >&2; exit 1; }

echo "fusion : squash seulement, auto-merge, branches supprimées après fusion"
gh api -X PATCH "$R" --silent \
  -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
  -F allow_auto_merge=true -F delete_branch_on_merge=true \
  -f 'security_and_analysis[secret_scanning][status]=enabled' \
  -f 'security_and_analysis[secret_scanning_push_protection][status]=enabled'

echo "actions : jeton en lecture par défaut, approbation des PR de contributeurs externes"
gh api -X PUT "$R/actions/permissions/workflow" --silent \
  -f default_workflow_permissions=read -F can_approve_pull_request_reviews=false
gh api -X PUT "$R/actions/permissions/fork-pr-contributor-approval" --silent \
  -f approval_policy=all_external_contributors

for name in "${RULESETS[@]}"; do
  file="$ROOT/github/rulesets/$name.json"
  title=$(jq -r .name "$file")
  id=$(gh api "$R/rulesets" --jq ".[] | select(.name == \"$title\") | .id")
  if [[ -n $id ]]; then
    echo "règle $title : mise à jour"; gh api -X PUT "$R/rulesets/$id" --input "$file" --silent
  else
    echo "règle $title : création"; gh api -X POST "$R/rulesets" --input "$file" --silent
  fi
done

admin_id=$(gh api "users/$ADMIN_LOGIN" --jq .id)
for env in "${ENVS[@]}"; do
  echo "environnement $env : relecture de $ADMIN_LOGIN"
  jq -n --argjson id "$admin_id" \
    '{reviewers: [{type: "User", id: $id}], prevent_self_review: false, deployment_branch_policy: null}' |
    gh api -X PUT "$R/environments/$env" --input - --silent
done
echo "garde-fous appliqués à $ORG/$REPO"
