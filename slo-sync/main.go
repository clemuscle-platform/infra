// slo-sync : traduit les objectifs de service des applications (deploy/slo.yaml, livré en
// ConfigMap platform.io/slo=true) en alertes OpenObserve de taux de consommation du budget
// d'erreur (ADR-0010). Exécuté par un CronJob : un passage, puis sortie.
//
// Variables d'environnement :
//
//	O2_URL                 http://openobserve.platform-observability.svc:5080
//	O2_ORG                 organisation OpenObserve (défaut : default)
//	O2_TRACES_STREAM       flux de traces (défaut : default)
//	ZO_ROOT_USER_EMAIL     identifiants OpenObserve (Secret openobserve-root)
//	ZO_ROOT_USER_PASSWORD
//	ALERT_BRIDGE_URL       http://alert-bridge.platform-ops.svc.cluster.local:8080/hooks/generic
//	PLATFORM_TOKEN         jeton d'alert-bridge, placé dans l'en-tête X-Platform-Token
//	DRY_RUN                true : n'écrit rien dans OpenObserve, journalise le plan
//
// Code de sortie : 0 si tout est appliqué, 1 en cas d'erreur (fichier invalide compris).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var tracesStream = envOr("O2_TRACES_STREAM", "default")

type config struct {
	O2URL, O2Org, O2User, O2Pass string
	BridgeURL, BridgeToken       string
	DryRun                       bool
}

func loadConfig() (config, error) {
	c := config{
		O2URL:       envOr("O2_URL", "http://openobserve.platform-observability.svc:5080"),
		O2Org:       envOr("O2_ORG", "default"),
		O2User:      os.Getenv("ZO_ROOT_USER_EMAIL"),
		O2Pass:      os.Getenv("ZO_ROOT_USER_PASSWORD"),
		BridgeURL:   envOr("ALERT_BRIDGE_URL", "http://alert-bridge.platform-ops.svc.cluster.local:8080/hooks/generic"),
		BridgeToken: os.Getenv("PLATFORM_TOKEN"),
		DryRun:      os.Getenv("DRY_RUN") == "true",
	}
	if c.O2User == "" || c.O2Pass == "" {
		return c, errors.New("identifiants OpenObserve absents (ZO_ROOT_USER_EMAIL, ZO_ROOT_USER_PASSWORD)")
	}
	if len(c.BridgeToken) < 24 {
		return c, errors.New("PLATFORM_TOKEN absent ou trop court (24 caractères minimum)")
	}
	return c, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func run(ctx context.Context, cfg config, kube interface {
	ListSLO(context.Context) ([]sloSource, error)
}, api alertAPI, log *slog.Logger) error {
	sources, err := kube.ListSLO(ctx)
	if err != nil {
		// Sans la liste complète, impossible de savoir quoi supprimer : on n'applique rien.
		return err
	}
	desired, protected, errs := plan(sources)
	for _, e := range errs {
		log.Error("fichier SLO rejeté", "err", e)
	}
	folderID, err := api.EnsureFolder(ctx)
	if err != nil {
		return err
	}
	if !cfg.DryRun {
		if err := api.EnsureNotifier(ctx, cfg.BridgeURL, cfg.BridgeToken); err != nil {
			return err
		}
	}
	r, err := reconcile(ctx, api, folderID, desired, protected, cfg.DryRun, log)
	if err != nil {
		return err
	}
	for _, e := range r.Errors {
		log.Error("alerte non appliquée", "err", e)
	}
	log.Info("synchronisation terminée", "fichiers", len(sources), "alertes", len(desired),
		"créées", r.Created, "mises_a_jour", r.Updated, "supprimées", r.Deleted, "inchangées", r.Unchanged,
		"erreurs", len(errs)+len(r.Errors), "dry_run", cfg.DryRun)
	if len(errs)+len(r.Errors) > 0 {
		return errors.New("synchronisation incomplète (voir les erreurs ci-dessus)")
	}
	return nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		log.Error("configuration", "err", err)
		os.Exit(1)
	}
	kube, err := newInClusterKube()
	if err != nil {
		log.Error("API Kubernetes", "err", err)
		os.Exit(1)
	}
	api := &o2Client{base: cfg.O2URL, org: cfg.O2Org, user: cfg.O2User, pass: cfg.O2Pass, http: &http.Client{Timeout: 20 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := run(ctx, cfg, kube, api, log); err != nil {
		log.Error("échec", "err", err)
		os.Exit(1)
	}
}
