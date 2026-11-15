# 0003 — Une base, deux surcouches

**Statut** : proposée · 2026-11-15

## Contexte
Il y a deux cibles : le k3s local d'abord, Hetzner ensuite. La différence entre les deux doit tenir dans les surcouches.

## Décision
- `cluster/base/<composant>/` contient les HelmRelease, les CRD et la configuration **sans valeur propre à un environnement**. Les valeurs qui varient d'un environnement à l'autre sont des variables de substitution Flux (`${cluster_domain}`, `${s3_endpoint}`, `${storage_class}`, `${preview_max}`…).
- Les variables sont définies dans `cluster/<env>/cluster-vars.yaml`, une ConfigMap publique, et `cluster/<env>/cluster-secrets.sops.yaml`, chiffrée. Elles sont injectées par `postBuild.substituteFrom`.
- `cluster/local/` et `cluster/hetzner/` sont des points d'entrée Kustomize. Ils incluent la base, puis ajoutent ou retirent des composants (Garage ou external-dns, par exemple), et appliquent des patches.
- Chaque environnement a sa `FluxInstance`, qui pointe vers `cluster/<env>`.
- Un contrôle de la CI vérifie que la base ne contient aucune chaîne propre à un environnement (`localhost`, `hetzner`, `r2.cloudflarestorage`, IP).

## Alternatives écartées
- Branches Git par environnement : elles divergent et les fusions deviennent pénibles.
- Deux dépôts d'infrastructure : la base serait dupliquée.

## Conséquences
- `kustomize build cluster/local` et `flux build` doivent passer tous les deux en CI, pour chaque environnement.
- Ajouter un troisième environnement (`hetzner-3n`, par exemple) revient à ajouter un dossier de surcouche.
