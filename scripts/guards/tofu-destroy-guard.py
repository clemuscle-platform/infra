#!/usr/bin/env python3
"""Garde-fou OpenTofu (règles d'autonomie, ADR-0014) : échoue si le plan détruit ou remplace une ressource.

Usage : tofu show -json tfplan | tofu-destroy-guard.py
Écrit « destructive=true|false » dans $GITHUB_OUTPUT si la variable existe.
"""

import json
import os
import sys

plan = json.load(sys.stdin)
destructive = []
for rc in plan.get("resource_changes", []):
    actions = rc.get("change", {}).get("actions", [])
    if "delete" in actions:
        kind = "remplacement" if "create" in actions else "destruction"
        destructive.append(f"{kind} : {rc['address']}")

if os.environ.get("GITHUB_OUTPUT"):
    with open(os.environ["GITHUB_OUTPUT"], "a") as f:
        f.write(f"destructive={'true' if destructive else 'false'}\n")

if destructive:
    print("Plan OpenTofu DESTRUCTIF : validation humaine requise (environnement « hetzner-destroy »).")
    print("\n".join(f"  - {d}" for d in destructive))
    sys.exit(1)
print("tofu-destroy-guard : aucune destruction ni remplacement")
