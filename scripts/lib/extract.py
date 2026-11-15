"""Extrait un objet (kind, name) d'un flux YAML multi-documents et l'écrit sur stdout."""
import sys

import yaml

src, kind, name = sys.argv[1:4]
for doc in yaml.safe_load_all(open(src)):
    if doc and doc.get("kind") == kind and doc["metadata"]["name"] == name:
        yaml.safe_dump(doc, sys.stdout, sort_keys=False)
        sys.exit(0)
sys.exit(f"{kind}/{name} introuvable dans {src}")
