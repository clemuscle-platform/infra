# Cibles communes à tous les dépôts : test, build, deploy, logs.
# Aucune cible ne modifie un cluster, sauf « deploy », réservée à l'humain (confirmation tofu).
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
ENV ?= local
ENVS := local hetzner

.PHONY: help tools test build deploy logs validate validate-tofu test-policies test-rbac test-rbac-live \
        test-redaction scan fmt plan secrets-local agent-kubeconfig clean

help: ## Affiche cette aide
	@grep -hE '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

tools: ## Installe les outils figés dans ~/.local/bin
	scripts/install-tools.sh

test: validate validate-tofu test-policies test-rbac scan ## Tous les contrôles hors cluster (comme la CI)

build: ## Rend les manifestes de ENV dans out/ENV (sans rien appliquer)
	scripts/validate.sh $(ENV)

validate: ## kustomize/flux build + kubeconform pour chaque environnement présent
	@for e in $(ENVS); do if [[ -f cluster/$$e/runtime-info.yaml ]]; then scripts/validate.sh $$e; fi; done

test-policies: ## Tests des ValidatingAdmissionPolicy (CLI kyverno)
	scripts/test-policies.sh $(ENV)

test-rbac: ## Contrôle statique du rôle agent-reader
	scripts/test-rbac.sh

test-rbac-live: ## Contrôle du rôle agent-reader sur le cluster courant (kubectl auth can-i)
	scripts/test-rbac.sh --live

scan: ## Trivy (configuration) + Gitleaks
	trivy config --quiet --severity HIGH,CRITICAL --exit-code 1 --ignorefile .trivyignore.yaml \
	  --skip-dirs out --skip-dirs .cache --skip-dirs cluster/base/policies/tests \
	  --skip-dirs "tofu/envs/*/.terraform" .
	gitleaks dir . --no-banner --redact
	gitleaks git . --no-banner --redact

validate-tofu: ## tofu fmt -check + tofu validate (sans backend)
	scripts/validate-tofu.sh

fmt: ## Formate le code OpenTofu
	tofu fmt -recursive tofu

plan: ## tofu plan de ENV (lecture seule ; l'humain lance ensuite « make deploy »)
	tofu -chdir=tofu/envs/$(ENV) init -input=false
	tofu -chdir=tofu/envs/$(ENV) plan -input=false -out=tfplan

deploy: ## [HUMAIN] Applique le plan OpenTofu de ENV (serveur, k3s, amorçage Flux). Flux fait le reste.
	@echo "Application du plan OpenTofu pour ENV=$(ENV). Relire le plan avant de confirmer."
	tofu -chdir=tofu/envs/$(ENV) apply tfplan

logs: ## Journaux Flux récents (lecture seule)
	flux logs --all-namespaces --since=1h --level=info
	flux get all -A --status-selector ready=false

secrets-local: ## Génère les secrets locaux aléatoires, chiffrés SOPS (n'écrase rien)
	scripts/gen-local-secrets.sh

agent-kubeconfig: ## [HUMAIN] Génère ~/.kube/agent-reader.yaml (jeton 24 h du compte agent-reader)
	scripts/agent-kubeconfig.sh

clean: ## Supprime les sorties de validation
	find out -type f -delete 2>/dev/null || true
