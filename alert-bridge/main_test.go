package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorder struct{ calls []string }

func (r *recorder) Upsert(_ context.Context, org, repo string, a Alert) (string, error) {
	r.calls = append(r.calls, org+"/"+repo+" "+a.Status+" "+a.Fingerprint())
	return "ok", nil
}

const token = "jeton-de-test-suffisamment-long"

func newTestServer() (*server, *recorder) {
	rec := &recorder{}
	return &server{cfg: config{Token: token, Org: "org", DefaultRep: "infra"}, issues: rec, log: slog.New(slog.NewTextHandler(io.Discard, nil))}, rec
}

func post(t *testing.T, h http.Handler, path, tok, body string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("X-Platform-Token", tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

func TestAuthRequired(t *testing.T) {
	s, rec := newTestServer()
	h := s.handle(parseGeneric)
	if c := post(t, h, "/hooks/generic", "", `{"source":"x","alert":"y"}`); c != http.StatusUnauthorized {
		t.Fatalf("sans jeton : %d", c)
	}
	if c := post(t, h, "/hooks/generic", "mauvais", `{"source":"x","alert":"y"}`); c != http.StatusUnauthorized {
		t.Fatalf("mauvais jeton : %d", c)
	}
	if len(rec.calls) != 0 {
		t.Fatal("aucun ticket ne doit être créé sans authentification")
	}
}

func TestGenericRoutesToAppRepo(t *testing.T) {
	s, rec := newTestServer()
	h := s.handle(parseGeneric)
	code := post(t, h, "/hooks/generic", token, `{"source":"openobserve","alert":"slo-disponibilite","app":"hello","tier":"prod","severity":"critical","status":"firing"}`)
	if code != http.StatusOK || len(rec.calls) != 1 || !strings.HasPrefix(rec.calls[0], "org/hello firing") {
		t.Fatalf("code=%d calls=%v", code, rec.calls)
	}
}

func TestFingerprintStableAcrossMessages(t *testing.T) {
	a := Alert{Source: "openobserve", Name: "slo", App: "hello", Tier: "prod", Message: "1"}
	b := a
	b.Message = "2"
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("l'empreinte ne doit pas dépendre du message")
	}
	b.Tier = "staging"
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("l'empreinte doit dépendre du niveau")
	}
}

func TestPreviewIgnored(t *testing.T) {
	s, rec := newTestServer()
	code := post(t, s.handle(parseGeneric), "/hooks/generic", token, `{"source":"o","alert":"a","app":"hello","tier":"preview"}`)
	if code != http.StatusNoContent || len(rec.calls) != 0 {
		t.Fatalf("préversion : code=%d calls=%v", code, rec.calls)
	}
}

func TestInvalidAppRejected(t *testing.T) {
	s, _ := newTestServer()
	if c := post(t, s.handle(parseGeneric), "/hooks/generic", token, `{"source":"o","alert":"a","app":"../../etc"}`); c != http.StatusBadRequest {
		t.Fatalf("nom d'application dangereux accepté : %d", c)
	}
}

func TestFluxEventMapping(t *testing.T) {
	a, err := parseFlux([]byte(`{"involvedObject":{"kind":"Kustomization","name":"app-hello-prod","namespace":"flux-system"},"severity":"error","reason":"ReconciliationFailed","message":"x"}`))
	if err != nil || a.App != "hello" || a.Tier != "prod" || a.Severity != "critical" || a.Status != "firing" {
		t.Fatalf("%+v %v", a, err)
	}
	a, _ = parseFlux([]byte(`{"involvedObject":{"kind":"Kustomization","name":"app-my-app-pr-12"},"severity":"error"}`))
	if a.App != "my-app" || a.Tier != "preview" {
		t.Fatalf("préversion : %+v", a)
	}
	a, _ = parseFlux([]byte(`{"involvedObject":{"kind":"HelmRelease","name":"cert-manager"},"severity":"info"}`))
	if a.App != "" || a.Status != "resolved" {
		t.Fatalf("plateforme : %+v", a)
	}
}

func TestUptimeRobotStatuses(t *testing.T) {
	for in, want := range map[string]string{"down": "firing", "up": "resolved", "": "firing", "resolved": "resolved"} {
		a, err := parseGeneric([]byte(`{"source":"uptimerobot","alert":"https","status":"` + in + `"}`))
		if err != nil || a.Status != want {
			t.Fatalf("%q → %q (%v)", in, a.Status, err)
		}
	}
}
