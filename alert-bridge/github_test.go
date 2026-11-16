package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeGitHub : API GitHub minimale en mémoire (jeton d'installation, dépôts, tickets, commentaires).
type fakeGitHub struct {
	mu       sync.Mutex
	repos    map[string]bool
	issues   map[string][]map[string]any // repo -> tickets
	comments int
	closed   int
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := r.URL.Path
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/access_tokens"):
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ey") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "ghs_test", "expires_at": time.Now().Add(time.Hour)})
	case r.Header.Get("Authorization") != "Bearer ghs_test":
		w.WriteHeader(http.StatusUnauthorized)
	case r.Method == http.MethodGet && strings.Count(p, "/") == 3: // /repos/org/repo
		if !f.repos[strings.TrimPrefix(p, "/repos/")] {
			w.WriteHeader(http.StatusNotFound)
		}
	case r.Method == http.MethodGet && strings.HasSuffix(p, "/issues"):
		repo := strings.TrimSuffix(strings.TrimPrefix(p, "/repos/"), "/issues")
		label := r.URL.Query().Get("labels")
		out := []map[string]any{}
		for _, is := range f.issues[repo] {
			if is["state"] == "open" && strings.Contains(fmt.Sprint(is["labels"]), label) {
				out = append(out, is)
			}
		}
		_ = json.NewEncoder(w).Encode(out)
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/issues"):
		repo := strings.TrimSuffix(strings.TrimPrefix(p, "/repos/"), "/issues")
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		in["number"] = len(f.issues[repo]) + 1
		in["state"] = "open"
		f.issues[repo] = append(f.issues[repo], in)
		_ = json.NewEncoder(w).Encode(in)
	case r.Method == http.MethodPost && strings.HasSuffix(p, "/comments"):
		f.comments++
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPatch:
		for _, list := range f.issues {
			for _, is := range list {
				if strings.HasSuffix(p, fmt.Sprintf("/issues/%v", is["number"])) {
					is["state"] = "closed"
				}
			}
		}
		f.closed++
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func testKeyPEM(t *testing.T) []byte {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
}

func TestGitHubLifecycle(t *testing.T) {
	fake := &fakeGitHub{repos: map[string]bool{"org/hello": true, "org/infra": true}, issues: map[string][]map[string]any{}}
	ts := httptest.NewServer(fake)
	defer ts.Close()
	gh, err := newGitHub(config{AppID: "1", InstallID: "2", DefaultRep: "infra", KeyPEM: testKeyPEM(t)})
	if err != nil {
		t.Fatal(err)
	}
	gh.api = ts.URL
	ctx := context.Background()
	a := Alert{Source: "openobserve", Name: "slo", App: "hello", Tier: "prod", Severity: "critical", Status: "firing"}

	if got, err := gh.Upsert(ctx, "org", "hello", a); err != nil || got != "created #1" {
		t.Fatalf("création : %q %v", got, err)
	}
	if got, _ := gh.Upsert(ctx, "org", "hello", a); got != "commented #1" {
		t.Fatalf("doublon : %q (attendu un commentaire)", got)
	}
	a.Status = "resolved"
	if got, _ := gh.Upsert(ctx, "org", "hello", a); got != "closed #1" || fake.closed != 1 {
		t.Fatalf("résolution : %q", got)
	}
	if got, _ := gh.Upsert(ctx, "org", "hello", a); !strings.HasPrefix(got, "ignored") {
		t.Fatalf("résolution sans ticket ouvert : %q", got)
	}
	// Dépôt inconnu de l'App → repli sur infra.
	b := Alert{Source: "flux", Name: "x", App: "inconnue", Status: "firing", Severity: "warning"}
	if got, _ := gh.Upsert(ctx, "org", "inconnue", b); got != "created #1" || len(fake.issues["org/infra"]) != 1 {
		t.Fatalf("repli : %q %v", got, fake.issues)
	}
}
