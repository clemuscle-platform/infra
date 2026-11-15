# 0017 — Services externes : sonde, erreurs, e-mail, sauvegardes

**Statut** : proposée, **choix ouverts**

| Besoin | Options | Proposition | Intégration prévue |
|---|---|---|---|
| Sauvegardes hors hébergeur | Cloudflare R2 / Backblaze B2 | **R2** : pas de frais de sortie, le compte Cloudflare existe déjà pour le DNS | rclone générique (S3), on passe de l'un à l'autre en changeant l'endpoint |
| Sonde externe | UptimeRobot | UptimeRobot (offre gratuite, intervalle de 5 min) | moniteurs déclarés dans `cluster/apps/<app>.yaml` (`uptime: /healthz`) et synchronisés par un Job, ou créés à la main au départ ; webhook vers alert-bridge |
| Suivi des erreurs | Sentry SaaS / GlitchTip (SaaS ou hébergé) | **GlitchTip SaaS** ou Sentry offre gratuite. L'auto-hébergement est écarté (RAM) | SDK Sentry dans le modèle, activé si `SENTRY_DSN` est défini |
| E-mails transactionnels | Brevo / Resend | à choisir. Le modèle utilise du **SMTP générique** (`SMTP_HOST`, `SMTP_USER`…), donc le choix n'impose aucun changement de code | module `email` du modèle, sortie réseau `smtp` (587) autorisée |

À décider par l'utilisateur avant la mise en production, pas avant la phase 2.
