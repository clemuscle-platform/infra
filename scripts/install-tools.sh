#!/usr/bin/env bash
# Installe les outils de la plateforme dans ~/.local/bin, versions figées, sommes de contrôle vérifiées.
# Versions relevées le 2026-11-15 ; mises à jour par Renovate (regex manager sur ce fichier).
set -euo pipefail
BIN="$HOME/.local/bin"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
GH=https://github.com

FLUX=2.10.0
FLUX_OPERATOR=0.62.1
SOPS=3.13.3
KUBECONFORM=0.8.0
KUSTOMIZE=5.8.3
GITLEAKS=8.30.1
KYVERNO=1.20.0
TOFU=1.13.9
TRIVY=0.76.2
UV=0.13.8
SEMGREP=1.185.0
ACTIONLINT=1.7.12
COSIGN=3.1.4

# fetch <url> <sums-url> <asset-name>
fetch() {
  local url=$1 sums=$2 name=$3
  curl -fsSL -o "$WORK/$name" "$url"
  curl -fsSL -o "$WORK/$name.sums" "$sums"
  local want; want=$(grep -E "[ *]$name\$" "$WORK/$name.sums" | awk '{print $1}')
  [ -n "$want" ] || { echo "pas de somme pour $name"; exit 1; }
  echo "$want  $WORK/$name" | sha256sum -c --quiet
}
untar() { mkdir -p "$WORK/x"; tar -xzf "$WORK/$1" -C "$WORK/x"; install -m 0755 "$WORK/x/$2" "$BIN/$3"; rm -rf "$WORK/x"; }

a=flux_${FLUX}_linux_amd64.tar.gz
fetch $GH/fluxcd/flux2/releases/download/v$FLUX/$a $GH/fluxcd/flux2/releases/download/v$FLUX/flux_${FLUX}_checksums.txt $a
untar $a flux flux

for p in flux-operator flux-operator-mcp; do
  a=${p}_${FLUX_OPERATOR}_linux_amd64.tar.gz
  fetch $GH/controlplaneio-fluxcd/flux-operator/releases/download/v$FLUX_OPERATOR/$a \
        $GH/controlplaneio-fluxcd/flux-operator/releases/download/v$FLUX_OPERATOR/flux-operator_${FLUX_OPERATOR}_checksums.txt $a
  untar $a $p $p
done

a=sops-v$SOPS.linux.amd64
fetch $GH/getsops/sops/releases/download/v$SOPS/$a $GH/getsops/sops/releases/download/v$SOPS/sops-v$SOPS.checksums.txt $a
install -m 0755 "$WORK/$a" "$BIN/sops"

a=kubeconform-linux-amd64.tar.gz
fetch $GH/yannh/kubeconform/releases/download/v$KUBECONFORM/$a $GH/yannh/kubeconform/releases/download/v$KUBECONFORM/CHECKSUMS $a
untar $a kubeconform kubeconform

a=kustomize_v${KUSTOMIZE}_linux_amd64.tar.gz
fetch $GH/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$KUSTOMIZE/$a $GH/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$KUSTOMIZE/checksums.txt $a
untar $a kustomize kustomize

a=gitleaks_${GITLEAKS}_linux_x64.tar.gz
fetch $GH/gitleaks/gitleaks/releases/download/v$GITLEAKS/$a $GH/gitleaks/gitleaks/releases/download/v$GITLEAKS/gitleaks_${GITLEAKS}_checksums.txt $a
untar $a gitleaks gitleaks

a=kyverno-cli_v${KYVERNO}_linux_x86_64.tar.gz
fetch $GH/kyverno/kyverno/releases/download/v$KYVERNO/$a $GH/kyverno/kyverno/releases/download/v$KYVERNO/checksums.txt $a
untar $a kyverno kyverno

a=tofu_${TOFU}_linux_amd64.tar.gz
fetch $GH/opentofu/opentofu/releases/download/v$TOFU/$a $GH/opentofu/opentofu/releases/download/v$TOFU/tofu_${TOFU}_SHA256SUMS $a
untar $a tofu tofu

a=trivy_${TRIVY}_Linux-64bit.tar.gz
fetch $GH/aquasecurity/trivy/releases/download/v$TRIVY/$a $GH/aquasecurity/trivy/releases/download/v$TRIVY/trivy_${TRIVY}_checksums.txt $a
untar $a trivy trivy

a=uv-x86_64-unknown-linux-gnu.tar.gz
curl -fsSL -o "$WORK/$a" $GH/astral-sh/uv/releases/download/$UV/$a
curl -fsSL -o "$WORK/$a.sha256" $GH/astral-sh/uv/releases/download/$UV/$a.sha256
echo "$(awk '{print $1}' "$WORK/$a.sha256")  $WORK/$a" | sha256sum -c --quiet
untar $a uv-x86_64-unknown-linux-gnu/uv uv

"$BIN/uv" tool install --force "semgrep==$SEMGREP"

a=actionlint_${ACTIONLINT}_linux_amd64.tar.gz
fetch $GH/rhysd/actionlint/releases/download/v$ACTIONLINT/$a $GH/rhysd/actionlint/releases/download/v$ACTIONLINT/actionlint_${ACTIONLINT}_checksums.txt $a
untar $a actionlint actionlint

a=cosign-linux-amd64
fetch $GH/sigstore/cosign/releases/download/v$COSIGN/$a $GH/sigstore/cosign/releases/download/v$COSIGN/cosign_checksums.txt $a
install -m 0755 "$WORK/$a" "$BIN/cosign"

echo "OK"
