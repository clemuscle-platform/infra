#!/usr/bin/env bash
# Teste les ValidatingAdmissionPolicy hors cluster avec la CLI kyverno.
# Les règles contiennent des variables Flux : on les rend avec les valeurs de l'environnement local.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV="${1:-local}"
T=cluster/base/policies/tests
mkdir -p "$T/.rendered"
# shellcheck disable=SC2046
export $(scripts/runtime-vars.sh "$ENV")
flux envsubst --strict < cluster/base/policies/app-policies.yaml > "$T/.rendered/app-policies.yaml"
kyverno test "$T" --remove-color --require-tests
