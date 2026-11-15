#!/usr/bin/env bash
# tofu fmt -check + tofu validate de chaque environnement, sans backend ni accès au cloud.
set -euo pipefail
cd "$(dirname "$0")/.."
tofu fmt -recursive -check -diff tofu
for env in tofu/envs/*/; do
  echo "== tofu validate $env"
  tofu -chdir="$env" init -backend=false -input=false -no-color >/dev/null
  tofu -chdir="$env" validate -no-color
done
