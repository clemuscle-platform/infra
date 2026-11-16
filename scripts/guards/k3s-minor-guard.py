#!/usr/bin/env python3
"""Garde-fou k3s (règles d'autonomie, ADR-0013) : échoue si une PR change la version MINEURE de k3s.

Compare les « version: vX.Y.Z+k3sN » des Plan SUC (cluster/hetzner/configs/upgrades/) et la variable
k3s_version d'OpenTofu entre la branche de base et HEAD. Un correctif (X.Y.z) passe ; une mineure
(X.Y → X.Y+1) exige l'environnement « k3s-minor ». Saut de plus d'une mineure : toujours refusé.
Usage : k3s-minor-guard.py [--base origin/main]
"""

import argparse
import os
import re
import subprocess
import sys

FILES = ["cluster/hetzner/configs/upgrades/k3s-server.yaml", "tofu/envs/hetzner/variables.tf"]
VERSION = re.compile(r"v(\d+)\.(\d+)\.(\d+)\+k3s\d+")


def versions(text: str) -> set[tuple[int, int]]:
    return {(int(m.group(1)), int(m.group(2))) for m in VERSION.finditer(text)}


def at(ref: str, path: str) -> str:
    r = subprocess.run(["git", "show", f"{ref}:{path}"], capture_output=True, text=True)  # noqa: S603,S607
    return r.stdout if r.returncode == 0 else ""


ap = argparse.ArgumentParser()
ap.add_argument("--base", default="origin/main")
base = ap.parse_args().base

minor_change, refused = [], []
for f in FILES:
    old, new = versions(at(base, f)), versions(open(f).read() if os.path.exists(f) else "")
    for o in old:
        for n in new:
            if n == o:
                continue
            if n[0] != o[0] or n[1] - o[1] > 1 or n < o:
                refused.append(f"{f} : {o[0]}.{o[1]} → {n[0]}.{n[1]} (une seule mineure à la fois, jamais en arrière)")
            elif n[1] == o[1] + 1:
                minor_change.append(f"{f} : {o[0]}.{o[1]} → {n[0]}.{n[1]}")

if os.environ.get("GITHUB_OUTPUT"):
    with open(os.environ["GITHUB_OUTPUT"], "a") as fh:
        fh.write(f"minor={'true' if minor_change else 'false'}\n")
if refused:
    print("Changement de version k3s refusé :\n  " + "\n  ".join(refused))
    sys.exit(2)
if minor_change:
    print("Montée de version MINEURE de k3s : validation humaine requise (environnement « k3s-minor »).")
    print("  " + "\n  ".join(minor_change))
    sys.exit(1)
print("k3s-minor-guard : pas de changement de version mineure")
