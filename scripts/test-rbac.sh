#!/usr/bin/env bash
# Vérifie que le rôle agent-reader est bien en lecture seule et sans accès aux Secrets (ADR-0014).
#   scripts/test-rbac.sh          contrôle statique du manifeste (CI, sans cluster)
#   scripts/test-rbac.sh --live   contrôle sur le cluster courant avec « kubectl auth can-i » (lecture seule)
set -euo pipefail
cd "$(dirname "$0")/.."
ROLE=cluster/base/configs/agents-rbac/agent-reader.yaml

python3 -I - "$ROLE" <<'PY'
import sys, yaml
FORBIDDEN_RES = {"secrets", "pods/exec", "pods/attach", "pods/portforward", "serviceaccounts/token",
                 "nodes/proxy", "services/proxy", "pods/proxy"}
ALLOWED_VERBS = {"get", "list", "watch"}
errors = []
for doc in yaml.safe_load_all(open(sys.argv[1])):
    if not doc or doc["kind"] != "ClusterRole":
        continue
    for i, rule in enumerate(doc["rules"]):
        res, verbs = set(rule.get("resources", [])), set(rule.get("verbs", []))
        if "*" in res or "*" in verbs or "*" in rule.get("apiGroups", []):
            errors.append(f"règle {i} : joker « * » interdit")
        if res & FORBIDDEN_RES:
            errors.append(f"règle {i} : ressources interdites {sorted(res & FORBIDDEN_RES)}")
        if verbs - ALLOWED_VERBS:
            errors.append(f"règle {i} : verbes interdits {sorted(verbs - ALLOWED_VERBS)}")
        if "impersonate" in verbs or "escalate" in verbs or "bind" in verbs:
            errors.append(f"règle {i} : verbe dangereux")
if errors:
    print("\n".join(errors)); sys.exit(1)
print("agent-reader (statique) : OK")
PY

[[ "${1:-}" == "--live" ]] || exit 0

AS=system:serviceaccount:platform-agents:agent-reader
fail=0
check() { # check <attendu yes|no> <verbe> <ressource> [-n ns]
  local want=$1; shift
  local got; got=$(kubectl auth can-i --as="$AS" "$@" 2>/dev/null || true)
  if [[ $got != "$want" ]]; then echo "ÉCHEC : can-i $* = $got (attendu $want)"; fail=1; else echo "ok    can-i $* = $got"; fi
}
check yes list pods -A
check yes get pods/log -n flux-system
check yes list kustomizations.kustomize.toolkit.fluxcd.io -A
check yes list clusters.postgresql.cnpg.io -A
check no  get secrets -A
check no  list secrets -n flux-system
check no  create pods/exec -n flux-system
check no  create pods/portforward -n flux-system
check no  create serviceaccounts/token -n platform-agents
check no  delete pods -A
check no  patch kustomizations.kustomize.toolkit.fluxcd.io -n flux-system
check no  impersonate serviceaccounts
exit $fail
