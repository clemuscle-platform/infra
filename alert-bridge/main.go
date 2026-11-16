// alert-bridge : transforme les alertes (OpenObserve, Flux, UptimeRobot) en tickets GitHub (ADR-0014).
//
// Points d'entrée (authentification : en-tête X-Platform-Token, comparaison à temps constant) :
//
//	POST /hooks/generic  schéma commun (OpenObserve, UptimeRobot, tout émetteur configurable)
//	POST /hooks/flux     événements du notification-controller de Flux (Provider « generic »)
//	GET  /healthz
//
// Dédoublonnage : une empreinte (source + alerte + application) devient le label « fp:<empreinte> ».
// Ticket ouvert avec la même empreinte → commentaire ; sinon création. « resolved » → commentaire + fermeture.
// Dépôt cible : clemuscle-platform/<app> si l'application est connue, sinon clemuscle-platform/infra.
// MODE=dry-run (ou DRY_RUN=true) : rien n'est envoyé à GitHub, les actions sont journalisées (environnement local).
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	Addr       string
	Token      string
	Org        string
	DefaultRep string
	DryRun     bool
	AppID      string
	InstallID  string
	KeyPEM     []byte
}

func loadConfig() (config, error) {
	c := config{
		Addr:       envOr("LISTEN_ADDR", ":8080"),
		Token:      os.Getenv("PLATFORM_TOKEN"),
		Org:        envOr("GITHUB_ORG", "clemuscle-platform"),
		DefaultRep: envOr("DEFAULT_REPO", "infra"),
		DryRun:     os.Getenv("MODE") == "dry-run" || os.Getenv("DRY_RUN") == "true",
		AppID:      os.Getenv("GITHUB_APP_ID"),
		InstallID:  os.Getenv("GITHUB_APP_INSTALLATION_ID"),
	}
	if len(c.Token) < 24 {
		return c, errors.New("PLATFORM_TOKEN absent ou trop court (24 caractères minimum)")
	}
	if !c.DryRun {
		path := envOr("GITHUB_APP_PRIVATE_KEY_FILE", "/var/run/secrets/github-app/private-key.pem")
		key, err := os.ReadFile(path)
		if err != nil {
			return c, fmt.Errorf("clé de la GitHub App illisible : %w", err)
		}
		c.KeyPEM = key
		if c.AppID == "" || c.InstallID == "" {
			return c, errors.New("GITHUB_APP_ID et GITHUB_APP_INSTALLATION_ID requis hors DRY_RUN")
		}
	}
	return c, nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

type server struct {
	cfg    config
	issues issueTracker
	log    *slog.Logger
}

func (s *server) authorized(r *http.Request) bool {
	got := r.Header.Get("X-Platform-Token")
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.Token)) == 1
}

func (s *server) handle(parse func([]byte) (Alert, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "méthode non autorisée", http.StatusMethodNotAllowed)
			return
		}
		if !s.authorized(r) {
			http.Error(w, "non autorisé", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 256<<10))
		if err != nil {
			http.Error(w, "lecture impossible", http.StatusBadRequest)
			return
		}
		alert, err := parse(body)
		if err != nil {
			s.log.Warn("alerte invalide", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if alert.Ignored() {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		action, err := s.route(ctx, alert)
		if err != nil {
			s.log.Error("échec du ticket", "fp", alert.Fingerprint(), "err", err)
			http.Error(w, "échec GitHub", http.StatusBadGateway)
			return
		}
		s.log.Info("alerte traitée", "action", action, "fp", alert.Fingerprint(), "app", alert.App, "alert", alert.Name)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"action": action, "fingerprint": alert.Fingerprint()})
	}
}

func (s *server) route(ctx context.Context, a Alert) (string, error) {
	repo := s.cfg.DefaultRep
	if a.App != "" {
		repo = a.App
	}
	return s.issues.Upsert(ctx, s.cfg.Org, repo, a)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("configuration", "err", err)
		os.Exit(1)
	}
	var tracker issueTracker = &dryRunTracker{log: logger}
	if !cfg.DryRun {
		gh, err := newGitHub(cfg)
		if err != nil {
			logger.Error("GitHub App", "err", err)
			os.Exit(1)
		}
		tracker = gh
	}
	s := &server{cfg: cfg, issues: tracker, log: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/hooks/generic", s.handle(parseGeneric))
	mux.HandleFunc("/hooks/flux", s.handle(parseFlux))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	srv := &http.Server{Addr: cfg.Addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}

	go func() {
		logger.Info("alert-bridge démarré", "addr", cfg.Addr, "dry_run", cfg.DryRun)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("serveur", "err", err)
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
