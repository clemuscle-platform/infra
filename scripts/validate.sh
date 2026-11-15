#!/usr/bin/env bash
# Validation hors cluster d'un environnement : rien n'est appliqué.
#   1. kustomize build de chaque couche ;
#   2. flux build de chaque couche, avec les variables de cluster/<env>/runtime-info.yaml ;
#   3. rendu des ResourceSet (app-tenants) pour chaque déclaration de cluster/apps ;
#   4. variables non résolues, Secrets non chiffrés, valeurs d'environnement dans la base ;
#   5. kubeconform (schémas Kubernetes + CRD).
# Usage : scripts/validate.sh <local|hetzner>
set -euo pipefail
cd "$(dirname "$0")/.."
ENV="${1:?usage: validate.sh <local|hetzner>}"
OUT="out/$ENV"
mkdir -p "$OUT"
K8S_VERSION="${K8S_VERSION:-1.35.0}"
fail=0
step() { printf '\n== %s\n' "$*"; }
err() { echo "ERREUR: $*" >&2; fail=1; }

# Couches appliquées par Flux : nom -> chemin (cf. cluster/base/platform/layers.yaml)
declare -A LAYERS=(
  [root]="cluster/$ENV"
  [platform-controllers]="cluster/$ENV/controllers"
  [platform-configs]="cluster/$ENV/configs"
  [platform-policies]="cluster/base/policies"
  [platform-tenants]="cluster/base/tenants"
  [apps]="cluster/apps"
)

mapfile -t VARS < <(scripts/runtime-vars.sh "$ENV")

step "kustomize build ($ENV)"
for layer in "${!LAYERS[@]}"; do
  kustomize build "${LAYERS[$layer]}" > "$OUT/$layer.kustomize.yaml" || err "kustomize build ${LAYERS[$layer]}"
  echo "ok  ${LAYERS[$layer]}"
done

step "flux build ($ENV)"
for layer in "${!LAYERS[@]}"; do
  ks="$OUT/$layer.ks.yaml"
  {
    echo "apiVersion: kustomize.toolkit.fluxcd.io/v1"
    echo "kind: Kustomization"
    echo "metadata: {name: $layer, namespace: flux-system}"
    echo "spec:"
    echo "  interval: 10m"
    echo "  path: ./${LAYERS[$layer]}"
    echo "  prune: true"
    echo "  sourceRef: {kind: GitRepository, name: flux-system}"
    echo "  postBuild:"
    echo "    substitute:"
    for kv in "${VARS[@]}"; do echo "      ${kv%%=*}: \"${kv#*=}\""; done
  } > "$ks"
  flux build kustomization "$layer" --path "./${LAYERS[$layer]}" --kustomization-file "$ks" --dry-run \
    > "$OUT/$layer.flux.yaml" 2> "$OUT/$layer.err" || { cat "$OUT/$layer.err"; err "flux build $layer"; }
  echo "ok  $layer"
done

step "rendu des ResourceSet app-tenants ($ENV)"
python3 -I scripts/lib/split_inputs.py "$OUT/apps.flux.yaml" "$OUT/inputs" "$ENV"
: > "$OUT/tenants-rendered.yaml"
for f in "$OUT"/inputs/*.yaml; do
  [[ -e $f ]] || continue
  python3 -I scripts/lib/extract.py "$OUT/platform-tenants.flux.yaml" ResourceSet app-tenants > "$OUT/app-tenants.rs.yaml"
  flux-operator build resourceset -f "$OUT/app-tenants.rs.yaml" --inputs-from "$f" >> "$OUT/tenants-rendered.yaml" \
    || err "flux-operator build resourceset ($f)"
  echo "---" >> "$OUT/tenants-rendered.yaml"
  echo "ok  $(basename "$f")"
done

step "contrôles de cohérence"
# Mode strict de Flux : toute variable doit être définie dans runtime-info (ou avoir une valeur par défaut).
printf '%s\n' "${VARS[@]}" > "$OUT/runtime-vars.txt"
python3 -I scripts/lib/check_vars.py "$OUT/runtime-vars.txt" "$OUT"/*.kustomize.yaml || err "substitution stricte"
# Variables encore présentes après flux build (hors objets « substitute: disabled »).
if grep -nE '\$\{[A-Z][A-Z0-9_]*\}' "$OUT"/*.flux.yaml "$OUT"/tenants-rendered.yaml; then
  err "variables non résolues ci-dessus"
fi
# Secrets : tout fichier contenant kind: Secret avec data/stringData doit être chiffré par SOPS.
while IFS= read -r f; do
  if grep -qE '^kind: Secret' "$f" && grep -qE '^(data|stringData):' "$f" && ! grep -q 'ENC\[AES256_GCM' "$f"; then
    err "Secret non chiffré : $f"
  fi
done < <(git ls-files --cached --others --exclude-standard 'cluster/**/*.yaml')
# La base ne doit contenir aucune valeur propre à un environnement (ADR-0003).
if grep -rnE '\blocalhost\b|hetzner|hcloud|r2\.cloudflarestorage|garage\.|local-path|[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}' cluster/base \
     --include='*.yaml' | grep -vE '^\S+:\s*#|0\.0\.0\.0/0|10\.0\.0\.0/8|172\.16\.0\.0/12|192\.168\.0\.0/16|169\.254\.0\.0/16|100\.64\.0\.0/10|/tests/' ; then
  err "valeurs d'environnement dans cluster/base (ci-dessus)"
fi
echo "ok"

step "kubeconform ($ENV, Kubernetes $K8S_VERSION)"
SCHEMAS=(-schema-location default
         -schema-location '.schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json'
         -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json')
mkdir -p .cache/kubeconform
kubeconform -strict -summary -kubernetes-version "$K8S_VERSION" -cache .cache/kubeconform \
  "${SCHEMAS[@]}" -skip CustomResourceDefinition \
  "$OUT"/*.flux.yaml "$OUT"/tenants-rendered.yaml \
  || err "kubeconform"

if (( fail )); then echo -e "\nÉCHEC de la validation ($ENV)"; exit 1; fi
echo -e "\nValidation $ENV : OK"
