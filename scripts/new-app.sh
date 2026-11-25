#!/usr/bin/env bash
# Crée une application de bout en bout, sous l'identité clemuscle-platform-bot (ADR-0014, ADR-0015) :
#   1. dépôt public <org>/<nom> depuis app-template ;
#   2. secrets de CI du dépôt (App clemuscle-platform-ci) ;
#   3. initialisation (scripts/init.sh) et premier commit sur main : la CI publie l'artefact « main » ;
#   4. garde-fous GitHub (scripts/github-guardrails.sh app) ;
#   5. PR de déclaration sur infra (cluster/apps/<nom>.yaml) : seule étape soumise à relecture humaine.
# Après la fusion de cette PR, Flux déploie le staging ; tout le reste passe par des PR sur le dépôt de l'application.
# Usage : scripts/new-app.sh <nom> [web|api|static]
# Le clone de l'application est créé dans $APPS_DIR/<nom> (par défaut : à côté d'infra).
set -euo pipefail

NAME=${1:?usage: $0 <nom> [web|api|static]}
KIND=${2:-web}
[[ $NAME =~ ^[a-z][a-z0-9-]{1,19}$ ]] || { echo "nom invalide : $NAME ([a-z][a-z0-9-]{1,19})" >&2; exit 2; }
[[ $KIND =~ ^(web|api|static)$ ]] || { echo "type invalide : $KIND" >&2; exit 2; }

ORG=${GITHUB_ORG:-clemuscle-platform}
INFRA=$(cd "$(dirname "$0")/.." && pwd)
APPS_DIR=${APPS_DIR:-$(dirname "$INFRA")}
CONF=${PLATFORM_CONFIG_DIR:-$HOME/.config/platform}
CI_KEY="$CONF/platform-ci-app.pem"
CI_CLIENT_ID=${PLATFORM_CI_CLIENT_ID:-Iv23liGUBuBfc8Eqpw4B}
DEST="$APPS_DIR/$NAME"

[[ -r $CI_KEY ]] || { echo "clé de l'App CI introuvable : $CI_KEY" >&2; exit 1; }
[[ ! -e $DEST ]] || { echo "$DEST existe déjà" >&2; exit 1; }
[[ ! -e $INFRA/cluster/apps/$NAME.yaml ]] || { echo "application déjà déclarée dans infra" >&2; exit 1; }

GH_TOKEN=$("$INFRA/scripts/bot-token.sh"); export GH_TOKEN
BOT_LOGIN="clemuscle-platform-bot[bot]"
BOT_ID=$(gh api "users/clemuscle-platform-bot%5Bbot%5D" --jq .id)
export GIT_AUTHOR_NAME=$BOT_LOGIN GIT_COMMITTER_NAME=$BOT_LOGIN
export GIT_AUTHOR_EMAIL="$BOT_ID+$BOT_LOGIN@users.noreply.github.com" GIT_COMMITTER_EMAIL="$BOT_ID+$BOT_LOGIN@users.noreply.github.com"
# git passe par gh, qui lit GH_TOKEN : le jeton n'est écrit dans aucun fichier.
git_() { git -c credential.helper= -c "credential.helper=!gh auth git-credential" "$@"; }

if gh api "repos/$ORG/$NAME" --silent 2>/dev/null; then
  echo "le dépôt $ORG/$NAME existe déjà" >&2; exit 1
fi

echo "1/5 dépôt $ORG/$NAME depuis app-template"
gh repo create "$ORG/$NAME" --public --template "$ORG/app-template" \
  --description "Application $NAME de la plateforme $ORG" >/dev/null
for _ in $(seq 1 30); do
  gh api "repos/$ORG/$NAME/branches/main" --silent 2>/dev/null && break
  sleep 2
done

echo "2/5 secrets de CI"
gh secret set PLATFORM_CI_CLIENT_ID -R "$ORG/$NAME" --body "$CI_CLIENT_ID"
gh secret set PLATFORM_CI_APP_PRIVATE_KEY -R "$ORG/$NAME" < "$CI_KEY"

echo "3/5 initialisation ($KIND) et premier commit"
git_ clone -q "https://github.com/$ORG/$NAME.git" "$DEST"
(
  cd "$DEST"
  git config user.name "$BOT_LOGIN"; git config user.email "$GIT_AUTHOR_EMAIL"
  scripts/init.sh "$NAME" "$KIND"
  git add -A
  git commit -qm "feat: initialisation de l'application $NAME"
  git_ push -q origin main
)

echo "4/5 garde-fous GitHub"
"$INFRA/scripts/github-guardrails.sh" app "$NAME"

echo "5/5 PR de déclaration sur infra"
WORK=$(mktemp -d); trap 'rm -rf "$WORK"' EXIT
git_ clone -q --depth 1 "https://github.com/$ORG/infra.git" "$WORK/infra"
(
  cd "$WORK/infra"
  git switch -q -c "app/$NAME"
  sed "s/hello/$NAME/g" cluster/apps/hello.yaml > "cluster/apps/$NAME.yaml"
  echo "  - $NAME.yaml" >> cluster/apps/kustomization.yaml
  git add cluster/apps
  git commit -qm "feat(apps): déclaration de l'application $NAME"
  git_ push -q -u origin "app/$NAME"
  gh pr create -R "$ORG/infra" --head "app/$NAME" \
    --title "Déclaration de l'application $NAME" \
    --body "Nouvelle application \`$NAME\` ($KIND), dépôt https://github.com/$ORG/$NAME. Niveaux : staging et prod, aucune sortie Internet. Après fusion, Flux déploie le staging (\`https://$NAME-staging.<domaine>\`). Relecture humaine requise (ADR-0014)."
)
echo "Fait. Clone local : $DEST"
