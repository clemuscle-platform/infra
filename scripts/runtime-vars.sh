#!/usr/bin/env bash
# Affiche les variables de cluster/<env>/runtime-info.yaml sous la forme CLE=valeur (une par ligne),
# pour reproduire hors cluster la substitution faite par Flux.
set -euo pipefail
cd "$(dirname "$0")/.."
ENV="${1:?usage: runtime-vars.sh <local|hetzner>}"
python3 -I - "cluster/$ENV/runtime-info.yaml" <<'PY'
import sys, yaml
data = yaml.safe_load(open(sys.argv[1]))["data"]
for k, v in data.items():
    if any(c.isspace() for c in str(v)):
        sys.exit(f"valeur avec espace non prise en charge : {k}")
    print(f"{k}={v}")
PY
