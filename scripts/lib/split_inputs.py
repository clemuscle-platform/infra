"""Transforme les déclarations d'applications (ResourceSetInputProvider) en fichiers d'entrées
pour `flux-operator build resourceset --inputs-from`.

- type Static            : une entrée = defaultValues
- type GitHubPullRequest : une entrée de démonstration (PR n° 1) = defaultValues + id/sha/branch
"""
import pathlib
import sys

import yaml

src, out_dir, _env = sys.argv[1], pathlib.Path(sys.argv[2]), sys.argv[3]
out_dir.mkdir(parents=True, exist_ok=True)
for old in out_dir.glob("*.yaml"):
    old.unlink()

for doc in yaml.safe_load_all(open(src)):
    if not doc or doc.get("kind") != "ResourceSetInputProvider":
        continue
    name = doc["metadata"]["name"]
    spec = doc["spec"]
    values = dict(spec.get("defaultValues") or {})
    if spec["type"] == "GitHubPullRequest":
        values.update(id="1", sha="0" * 40, branch="exemple", title="PR de démonstration", author="ci")
    elif spec["type"] != "Static":
        sys.exit(f"type non pris en charge par la validation : {spec['type']} ({name})")
    values.setdefault("id", name)
    (out_dir / f"{name}.yaml").write_text(yaml.safe_dump([values]))
