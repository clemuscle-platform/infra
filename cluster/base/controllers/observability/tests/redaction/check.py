"""Vérifie la sortie du collecteur : aucun secret ni donnée personnelle ne doit subsister.

Les valeurs sensibles de input.jsonl sont toutes fausses (préfixe FAUX, domaines example.org).
"""

import json
import sys

out_path, input_path = sys.argv[1], sys.argv[2]

# Fragments qui ne doivent JAMAIS apparaître en sortie.
FORBIDDEN = [
    "alice.martin@example.org", "bob@example.org", "ghp_FAUX", "github_pat_FAUX", "AKIAFAUX",
    "eyJhbGciOiJIUzI1NiJ9", "MotDePasseFaux", "4111 1111 1111 1111", "FR76 3000", "06 12 34 56 78",
    "MIIFAUXFAUXFAUX", "ZmF1eDpmYXV4", "valeur-quelconque",
]
# Ce qui doit rester intact (pas de faux positif sur un horodatage ou une durée).
MUST_KEEP = {"harmless-body": ["12 ms", "1763241600000"]}

records = {}
with open(out_path) as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        doc = json.loads(line)
        for rl in doc.get("resourceLogs", []):
            for sl in rl.get("scopeLogs", []):
                for rec in sl.get("logRecords", []):
                    attrs = {a["key"]: a["value"] for a in rec.get("attributes", [])}
                    case = attrs.get("case", {}).get("stringValue", "?")
                    records[case] = json.dumps(rec, ensure_ascii=False)

expected_cases = {json.loads(line)["case"] for line in open(input_path) if line.strip()}
errors = []
missing = expected_cases - set(records)
if missing:
    errors.append(f"enregistrements absents de la sortie : {sorted(missing)}")
for case, text in records.items():
    for frag in FORBIDDEN:
        if frag in text:
            errors.append(f"{case} : « {frag} » non masqué")
    for frag in MUST_KEEP.get(case, []):
        if frag not in text:
            errors.append(f"{case} : « {frag} » masqué à tort (faux positif)")

if errors:
    print("ÉCHEC du test de masquage :\n  " + "\n  ".join(errors))
    sys.exit(1)
print(f"masquage OTel : OK ({len(records)} enregistrements, {len(FORBIDDEN)} motifs interdits absents)")
