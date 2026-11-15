#!/usr/bin/env bash
# [HUMAIN] Crée ~/.kube/agent-reader.yaml : kubeconfig du compte agent-reader (lecture seule, sans Secrets),
# avec un jeton de 24 h. Utilisé par les agents et par le serveur MCP Flux (--read-only).
# Nécessite les droits administrateur du kubeconfig courant (création d'un TokenRequest).
set -euo pipefail
OUT="${1:-$HOME/.kube/agent-reader.yaml}"
DURATION="${DURATION:-24h}"
ctx=$(kubectl config current-context)
cluster=$(kubectl config view -o jsonpath="{.contexts[?(@.name==\"$ctx\")].context.cluster}")
server=$(kubectl config view -o jsonpath="{.clusters[?(@.name==\"$cluster\")].cluster.server}")
ca=$(kubectl config view --raw -o jsonpath="{.clusters[?(@.name==\"$cluster\")].cluster.certificate-authority-data}")
token=$(kubectl create token agent-reader -n platform-agents --duration="$DURATION")
umask 077
cat > "$OUT" <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: $cluster
    cluster:
      server: $server
      certificate-authority-data: $ca
users:
  - name: agent-reader
    user:
      token: $token
contexts:
  - name: agent-reader@$cluster
    context: {cluster: $cluster, user: agent-reader}
current-context: agent-reader@$cluster
EOF
echo "écrit : $OUT (valide $DURATION)"
