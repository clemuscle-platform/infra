# 0005 — ValidatingAdmissionPolicy + Pod Security Admission plutôt que Kyverno

**Statut** : proposée · 2026-11-15

## Contexte
Kyverno ou ValidatingAdmissionPolicy (VAP), selon ce qui suffit et coûte le moins. Le budget est de 8 Go de RAM. VAP est en GA depuis Kubernetes 1.30. MutatingAdmissionPolicy n'est en GA qu'à partir de la 1.36, alors que le cluster local est en 1.35.

## Décision
- **Pod Security Admission** (natif), niveau `restricted` en `enforce`, sur tous les espaces de noms d'applications. Ce niveau couvre : pas de root, pas d'escalade de privilèges, seccomp `RuntimeDefault`, capacités retirées, pas de volumes hôte.
- Des **ValidatingAdmissionPolicy** (CEL) couvrent le reste, avec des bindings qui sélectionnent les espaces de noms `platform.io/managed=true` :
  1. images limitées à `ghcr.io/<org>/` (et à une courte liste autorisée pour la plateforme), pas de tag `latest`, digest exigé en production ;
  2. requests CPU et mémoire et limit mémoire obligatoires ;
  3. labels `app.kubernetes.io/name` et `platform.io/app` obligatoires ;
  4. Services `LoadBalancer` et `NodePort` interdits dans les espaces de noms d'applications ;
  5. Ingress : l'hôte doit être un sous-domaine attribué à l'application (`*.${cluster_domain}` avec préfixe `<app>`) ;
  6. `automountServiceAccountToken: false`, sauf annotation explicite ;
  7. pas de suppression d'un `Cluster` CNPG de production sans l'annotation `platform.io/allow-delete` (garde-fou sur les données).
- Aucune mutation : les valeurs par défaut sont écrites explicitement dans les manifestes du modèle, et les VAP refusent ce qui manque.
- Tests hors cluster avec **la CLI kyverno** (`kyverno test`, qui évalue les VAP), avec des jeux de ressources conformes et non conformes. Ils tournent dans la CI.

## Alternatives écartées
- Kyverno : il pèse environ 0,5 à 1 Go de RAM pour 4 contrôleurs, et un webhook en panne bloque le cluster. Sa génération de ressources (netpol, quota) est remplacée par le composant Kustomize « tenant » (ADR-0006).
- MutatingAdmissionPolicy : à reconsidérer une fois les deux environnements en 1.36 ou plus.

## Conséquences
- Les règles sont plus simples que celles de Kyverno, mais natives, sans webhook et sans coût mémoire.
- Une règle peut d'abord être déployée en `validationActions: [Warn, Audit]`, puis passer en `Deny`.
