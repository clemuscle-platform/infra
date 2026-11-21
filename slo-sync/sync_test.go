package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeAPI : OpenObserve en mémoire.
type fakeAPI struct {
	alerts           map[string]existingAlert
	created, updated []string
	deleted          []string
	notifierEnsured  bool
	failList         bool
}

func newFake(existing ...existingAlert) *fakeAPI {
	f := &fakeAPI{alerts: map[string]existingAlert{}}
	for _, e := range existing {
		f.alerts[e.Name] = e
	}
	return f
}

func (f *fakeAPI) EnsureFolder(context.Context) (string, error) { return "dossier", nil }
func (f *fakeAPI) EnsureNotifier(context.Context, string, string) error {
	f.notifierEnsured = true
	return nil
}
func (f *fakeAPI) ListAlerts(context.Context, string) ([]existingAlert, error) {
	if f.failList {
		return nil, errors.New("panne")
	}
	var out []existingAlert
	for _, a := range f.alerts {
		out = append(out, a)
	}
	return out, nil
}
func (f *fakeAPI) CreateAlert(_ context.Context, _ string, a AlertSpec) error {
	f.created = append(f.created, a.Name)
	f.alerts[a.Name] = existingAlert{ID: "id-" + a.Name, Name: a.Name, Tags: []string{managedTag, hashTagPfx + a.Hash}}
	return nil
}
func (f *fakeAPI) UpdateAlert(_ context.Context, _, id string, a AlertSpec) error {
	f.updated = append(f.updated, a.Name)
	f.alerts[a.Name] = existingAlert{ID: id, Name: a.Name, Tags: []string{managedTag, hashTagPfx + a.Hash}}
	return nil
}
func (f *fakeAPI) DeleteAlert(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeKube struct {
	sources []sloSource
	err     error
}

func (k fakeKube) ListSLO(context.Context) ([]sloSource, error) { return k.sources, k.err }

var cfg = config{BridgeURL: "http://bridge", BridgeToken: strings.Repeat("x", 24)}

func helloProd() sloSource {
	return sloSource{Namespace: "app-hello-prod", Name: "slo-abc", Data: []byte(templateSLO)}
}

func TestCreateThenIdempotent(t *testing.T) {
	api := newFake()
	if err := run(context.Background(), cfg, fakeKube{sources: []sloSource{helloProd()}}, api, quiet); err != nil {
		t.Fatal(err)
	}
	if len(api.created) != 4 || !api.notifierEnsured {
		t.Fatalf("4 créations attendues : %v", api.created)
	}
	api.created = nil
	if err := run(context.Background(), cfg, fakeKube{sources: []sloSource{helloProd()}}, api, quiet); err != nil {
		t.Fatal(err)
	}
	if len(api.created)+len(api.updated)+len(api.deleted) != 0 {
		t.Fatalf("second passage : rien ne doit changer (%v %v %v)", api.created, api.updated, api.deleted)
	}
}

func TestUpdateOnChange(t *testing.T) {
	api := newFake()
	_ = run(context.Background(), cfg, fakeKube{sources: []sloSource{helloProd()}}, api, quiet)
	changed := helloProd()
	changed.Data = []byte(strings.Replace(templateSLO, "threshold_ms: 400", "threshold_ms: 800", 1))
	_ = run(context.Background(), cfg, fakeKube{sources: []sloSource{changed}}, api, quiet)
	if len(api.updated) != 2 {
		t.Fatalf("2 mises à jour attendues (latence rapide et lente) : %v", api.updated)
	}
}

func TestPruneOnlyManaged(t *testing.T) {
	api := newFake(
		existingAlert{ID: "1", Name: "slo-old-prod-x-fast", Tags: []string{managedTag}},
		existingAlert{ID: "2", Name: "alerte-manuelle", Tags: []string{"perso"}},
	)
	_ = run(context.Background(), cfg, fakeKube{sources: []sloSource{helloProd()}}, api, quiet)
	if len(api.deleted) != 1 || api.deleted[0] != "1" {
		t.Fatalf("seule l'alerte gérée doit être supprimée : %v", api.deleted)
	}
}

func TestInvalidFileProtectsItsAlerts(t *testing.T) {
	api := newFake(existingAlert{ID: "1", Name: "slo-hello-prod-disponibilite-fast", Tags: []string{managedTag}})
	broken := helloProd()
	broken.Data = []byte("kind: cassé")
	err := run(context.Background(), cfg, fakeKube{sources: []sloSource{broken}}, api, quiet)
	if err == nil {
		t.Fatal("un fichier invalide doit faire échouer le passage")
	}
	if len(api.deleted) != 0 {
		t.Fatalf("les alertes d'une application au fichier cassé ne doivent pas être supprimées : %v", api.deleted)
	}
}

func TestNothingAppliedWhenListingFails(t *testing.T) {
	api := newFake(existingAlert{ID: "1", Name: "slo-hello-prod-disponibilite-fast", Tags: []string{managedTag}})
	if err := run(context.Background(), cfg, fakeKube{err: errors.New("API indisponible")}, api, quiet); err == nil {
		t.Fatal("erreur attendue")
	}
	if len(api.deleted)+len(api.created) != 0 || api.notifierEnsured {
		t.Fatal("rien ne doit être appliqué si la liste des ConfigMaps échoue")
	}
}

func TestUnmanagedNameClashIsReported(t *testing.T) {
	api := newFake(existingAlert{ID: "1", Name: "slo-hello-prod-disponibilite-fast", Tags: nil})
	if err := run(context.Background(), cfg, fakeKube{sources: []sloSource{helloProd()}}, api, quiet); err == nil {
		t.Fatal("un conflit de nom avec une alerte non gérée doit être signalé")
	}
	if len(api.updated) != 0 {
		t.Fatal("une alerte non gérée ne doit jamais être modifiée")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	api := newFake(existingAlert{ID: "1", Name: "slo-old-prod-x-fast", Tags: []string{managedTag}})
	dry := cfg
	dry.DryRun = true
	if err := run(context.Background(), dry, fakeKube{sources: []sloSource{helloProd()}}, api, quiet); err != nil {
		t.Fatal(err)
	}
	if len(api.created)+len(api.deleted) != 0 || api.notifierEnsured {
		t.Fatal("DRY_RUN ne doit rien écrire")
	}
}

// Client HTTP contre un faux serveur : chemins, authentification et corps envoyés.
func TestO2ClientRequests(t *testing.T) {
	var seen []string
	var alertBodyJSON map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "u" || p != "p" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/default/folders/alerts/name/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/default/folders/alerts":
			_, _ = w.Write([]byte(`{"folderId":"F1","name":"slo","description":""}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/default/alerts":
			_ = json.NewDecoder(r.Body).Decode(&alertBodyJSON)
			_, _ = w.Write([]byte(`{"code":200}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	c := &o2Client{base: srv.URL, org: "default", user: "u", pass: "p", http: srv.Client()}
	id, err := c.EnsureFolder(context.Background())
	if err != nil || id != "F1" {
		t.Fatalf("dossier : %q %v", id, err)
	}
	f, _ := ParseSLO([]byte(templateSLO), "app-hello-prod")
	if err := c.CreateAlert(context.Background(), id, DesiredAlerts(f)[0]); err != nil {
		t.Fatal(err)
	}
	if last := seen[len(seen)-1]; last != "POST /api/v2/default/alerts?folder=F1" {
		t.Errorf("requête de création : %s", last)
	}
	if alertBodyJSON["stream_type"] != "traces" || alertBodyJSON["notify_on_recovery"] != true {
		t.Errorf("corps de l'alerte inattendu : %v", alertBodyJSON)
	}
	if attrs, _ := alertBodyJSON["context_attributes"].(map[string]any); attrs["app"] != "hello" {
		t.Errorf("context_attributes manquants : %v", alertBodyJSON["context_attributes"])
	}
}

func TestDecodeConfigMaps(t *testing.T) {
	raw := `{"metadata":{},"items":[{"metadata":{"name":"slo-x","namespace":"app-hello-prod"},"data":{"slo.yaml":"a: 1"}}]}`
	got, err := decodeConfigMaps([]byte(raw))
	if err != nil || len(got) != 1 || got[0].Namespace != "app-hello-prod" || string(got[0].Data) != "a: 1" {
		t.Fatalf("%v %v", got, err)
	}
	if _, err := decodeConfigMaps([]byte(`{"metadata":{"continue":"x"},"items":[]}`)); err == nil {
		t.Fatal("une liste paginée doit être refusée")
	}
}
