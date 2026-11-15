"""Reproduit hors cluster le mode StrictPostBuildSubstitutions de kustomize-controller.

Pour chaque sortie `kustomize build` d'une couche (avant substitution), vérifie que :
  - chaque ${VAR} sans valeur par défaut est défini dans runtime-info ;
  - aucune séquence ${...} invalide n'est présente (Flux refuse alors toute la couche).
Les objets annotés kustomize.toolkit.fluxcd.io/substitute: disabled sont ignorés, comme par Flux.
Usage : check_vars.py <runtime-vars-file> <kustomize-build.yaml>...
"""
import re
import sys

import yaml

defined = {line.split("=", 1)[0] for line in open(sys.argv[1]).read().split() if "=" in line}
VAR = re.compile(r"(?<!\$)\$\{([^}]*)\}")
NAME = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(:?[-=+?].*)?$")
errors = []
for path in sys.argv[2:]:
    for doc in yaml.safe_load_all(open(path)):
        if not doc:
            continue
        ann = (doc.get("metadata") or {}).get("annotations") or {}
        if ann.get("kustomize.toolkit.fluxcd.io/substitute") == "disabled":
            continue
        ident = f"{doc.get('kind')}/{(doc.get('metadata') or {}).get('name')}"
        for m in VAR.finditer(yaml.safe_dump(doc)):
            expr = m.group(1)
            if not NAME.match(expr):
                errors.append(f"{path}: {ident}: expression invalide ${{{expr}}}")
                continue
            name = re.match(r"[A-Za-z_][A-Za-z0-9_]*", expr).group(0)
            has_default = len(expr) > len(name)
            if name not in defined and not has_default:
                errors.append(f"{path}: {ident}: variable non définie {name}")
for e in sorted(set(errors)):
    print(e)
sys.exit(1 if errors else 0)
