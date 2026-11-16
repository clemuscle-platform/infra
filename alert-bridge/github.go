package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type issueTracker interface {
	Upsert(ctx context.Context, org, repo string, a Alert) (string, error)
}

// dryRunTracker : environnement local, aucune écriture sur GitHub.
type dryRunTracker struct{ log *slog.Logger }

func (d *dryRunTracker) Upsert(_ context.Context, org, repo string, a Alert) (string, error) {
	d.log.Info("DRY_RUN : ticket", "repo", org+"/"+repo, "title", a.Title(), "labels", a.Labels(), "resolved", a.Resolved())
	return "dry-run", nil
}

// gitHub : authentification GitHub App (JWT RS256 → jeton d'installation, renouvelé avant expiration).
type gitHub struct {
	cfg  config
	key  *rsa.PrivateKey
	http *http.Client
	api  string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func newGitHub(cfg config) (*gitHub, error) {
	block, _ := pem.Decode(cfg.KeyPEM)
	if block == nil {
		return nil, errors.New("clé PEM invalide")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k8, err8 := x509.ParsePKCS8PrivateKey(block.Bytes); err8 == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("la clé de la GitHub App doit être RSA")
		}
		key = rk
	} else {
		return nil, fmt.Errorf("clé illisible : %w", err)
	}
	return &gitHub{cfg: cfg, key: key, http: &http.Client{Timeout: 15 * time.Second}, api: "https://api.github.com"}, nil
}

func (g *gitHub) appJWT() (string, error) {
	now := time.Now()
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"iat": now.Add(-60 * time.Second).Unix(), "exp": now.Add(9 * time.Minute).Unix(), "iss": g.cfg.AppID})
	unsigned := header + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, g.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + enc.EncodeToString(sig), nil
}

func (g *gitHub) installationToken(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.token != "" && time.Until(g.expires) > 5*time.Minute {
		return g.token, nil
	}
	jwt, err := g.appJWT()
	if err != nil {
		return "", err
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	path := fmt.Sprintf("/app/installations/%s/access_tokens", g.cfg.InstallID)
	if err := g.do(ctx, http.MethodPost, path, "Bearer "+jwt, nil, &out); err != nil {
		return "", fmt.Errorf("jeton d'installation : %w", err)
	}
	g.token, g.expires = out.Token, out.ExpiresAt
	return g.token, nil
}

func (g *gitHub) do(ctx context.Context, method, path, auth string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.api+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "platform-alert-bridge")
	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s : %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (g *gitHub) Upsert(ctx context.Context, org, repo string, a Alert) (string, error) {
	tok, err := g.installationToken(ctx)
	if err != nil {
		return "", err
	}
	auth := "Bearer " + tok
	// Dépôt inconnu de l'App (application non installée) : repli sur le dépôt par défaut.
	if err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", org, repo), auth, nil, nil); err != nil {
		repo = g.cfg.DefaultRep
	}
	var open []struct {
		Number int `json:"number"`
	}
	q := url.Values{"state": {"open"}, "labels": {"fp:" + a.Fingerprint()}, "per_page": {"1"}}
	if err := g.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/issues?%s", org, repo, q.Encode()), auth, nil, &open); err != nil {
		return "", err
	}
	comment := fmt.Sprintf("**%s** (%s, %s)\n\n%s\n\n%s", strings.ToUpper(a.Status), a.Source, time.Now().UTC().Format(time.RFC3339), a.Message, a.URL)
	if len(open) > 0 {
		n := open[0].Number
		if err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", org, repo, n), auth, map[string]string{"body": comment}, nil); err != nil {
			return "", err
		}
		if a.Resolved() {
			if err := g.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/%s/issues/%d", org, repo, n), auth, map[string]string{"state": "closed", "state_reason": "completed"}, nil); err != nil {
				return "", err
			}
			return fmt.Sprintf("closed #%d", n), nil
		}
		return fmt.Sprintf("commented #%d", n), nil
	}
	if a.Resolved() {
		return "ignored (resolved sans ticket ouvert)", nil
	}
	body := fmt.Sprintf("Ticket ouvert par alert-bridge.\n\n- Source : %s\n- Application : %s\n- Niveau : %s\n- Sévérité : %s\n- Empreinte : `%s`\n\n%s\n\n%s\n\nProcédure : docs/runbooks/boucles-exploitation.md (boucle incident).",
		a.Source, orDash(a.App), orDash(a.Tier), a.Severity, a.Fingerprint(), a.Message, a.URL)
	var created struct {
		Number int `json:"number"`
	}
	in := map[string]any{"title": a.Title(), "body": body, "labels": a.Labels()}
	if err := g.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues", org, repo), auth, in, &created); err != nil {
		return "", err
	}
	return fmt.Sprintf("created #%d", created.Number), nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
