# 0007 — Secrets SOPS/age, une clé par environnement, état OpenTofu chiffré

**Statut** : proposée · 2026-11-15

## Décision
- Les secrets sont chiffrés dans Git avec **SOPS** (3.13), seulement sur les champs `data` et `stringData` (`encrypted_regex`). Flux les déchiffre (`decryption.provider: sops`) avec le Secret `flux-system/sops-age`.
- **Une clé age par environnement**, selon des règles de chemin dans `.sops.yaml` :
  - `cluster/local/**`, et les `deploy/**/local/**` des applications : `age18enzprddd0at7gzk4929j4x093u3z52lvmcjv308yua2clfsh5zqx46utu` ;
  - `cluster/hetzner/**` et `deploy/**` pour la production et le staging Hetzner : clé `hetzner`, emplacement réservé `AGE_HETZNER_PUBLIC_KEY`, générée plus tard ;
  - **une clé de secours** (hors ligne) ajoutée comme destinataire de tous les fichiers, pour pouvoir rechiffrer si une clé d'environnement est perdue.
- Les clés privées ne sont jamais dans Git ni dans GitHub. OpenTofu lit le fichier `keys.txt` localement et crée le Secret `sops-age` dans le cluster.
- L'**état OpenTofu est chiffré** avec le chiffrement natif d'OpenTofu (`encryption` + `key_provider "pbkdf2"`), qui prend sa phrase dans `TF_VAR_state_passphrase`. Le Secret `sops-age` est dans l'état, donc l'état doit être chiffré.
- Backend de l'état : fichier local pour `envs/local`, bucket S3 dédié (Hetzner Object Storage, versionné) pour `envs/hetzner`.
- Gitleaks tourne en pré-commit et dans la CI. Un contrôle de la CI refuse tout `kind: Secret` non chiffré par SOPS.

## Conséquences
- Les valeurs factices du dépôt sont chiffrées avec la clé locale. La liste des vrais secrets est dans `docs/secrets-de-depart.md`.
- Pour faire tourner une clé : `sops updatekeys` sur tous les fichiers, puis mise à jour du Secret par OpenTofu.
