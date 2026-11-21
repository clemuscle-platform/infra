package main

import (
	"strings"
	"testing"
)

// Copie du modèle d'app-template (deploy/base/slo.yaml) après substitution par Flux.
const templateSLO = `apiVersion: platform.io/v1
kind: ServiceLevelObjectives
service: hello
tier: prod
objectives:
  - name: disponibilite
    description: Part des requêtes HTTP sans erreur serveur (5xx)
    target: 99.5
    indicator:
      type: error-rate
      good: 'http.response.status_code < 500'
  - name: latence
    description: Temps de réponse au 95e centile
    target: 95
    threshold_ms: 400
    indicator:
      type: latency
      exclude_routes: ["/healthz", "/readyz"]
alerting:
  fast_burn: {window: 5m, burn_rate: 14.4}
  slow_burn: {window: 1h, burn_rate: 6}
  tiers: [prod, staging]
`

func withTier(tier string) string {
	return strings.Replace(templateSLO, "tier: prod", "tier: "+tier, 1)
}

func TestTemplateIsValid(t *testing.T) {
	f, err := ParseSLO([]byte(templateSLO), "app-hello-prod")
	if err != nil {
		t.Fatal(err)
	}
	alerts := DesiredAlerts(f)
	if len(alerts) != 4 {
		t.Fatalf("4 alertes attendues, %d obtenues", len(alerts))
	}
	got := map[string]string{}
	for _, a := range alerts {
		got[a.Name] = a.Attributes["severity"]
	}
	want := map[string]string{
		"slo-hello-prod-disponibilite-fast": "critical",
		"slo-hello-prod-disponibilite-slow": "warning",
		"slo-hello-prod-latence-fast":       "critical",
		"slo-hello-prod-latence-slow":       "warning",
	}
	for n, sev := range want {
		if got[n] != sev {
			t.Errorf("%s : sévérité %q, attendu %q", n, got[n], sev)
		}
	}
}

func TestSQL(t *testing.T) {
	f, _ := ParseSLO([]byte(templateSLO), "app-hello-prod")
	byName := map[string]AlertSpec{}
	for _, a := range DesiredAlerts(f) {
		byName[a.Name] = a
	}
	avail := byName["slo-hello-prod-disponibilite-fast"]
	wantAvail := `SELECT count(*) AS total, sum(CASE WHEN CAST(http_response_status_code AS INT) >= 500 THEN 1 ELSE 0 END) AS bad FROM "default" WHERE service_service_namespace = 'app-hello-prod' AND span_kind = '2' HAVING count(*) >= 20 AND sum(CASE WHEN CAST(http_response_status_code AS INT) >= 500 THEN 1 ELSE 0 END) > 0.072000 * count(*)`
	if avail.SQL != wantAvail {
		t.Errorf("SQL disponibilité :\n%s\nattendu :\n%s", avail.SQL, wantAvail)
	}
	if avail.PeriodMin != 5 || avail.EveryMin != 1 {
		t.Errorf("fenêtre rapide : period %d every %d", avail.PeriodMin, avail.EveryMin)
	}
	lat := byName["slo-hello-prod-latence-slow"]
	for _, frag := range []string{"duration > 400000", "http_route NOT IN ('/healthz', '/readyz')", "> 0.300000 * count(*)"} {
		if !strings.Contains(lat.SQL, frag) {
			t.Errorf("SQL latence sans %q : %s", frag, lat.SQL)
		}
	}
	if lat.PeriodMin != 60 || lat.EveryMin != 5 {
		t.Errorf("fenêtre lente : period %d every %d", lat.PeriodMin, lat.EveryMin)
	}
}

func TestTiers(t *testing.T) {
	f, err := ParseSLO([]byte(withTier("staging")), "app-hello-staging")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range DesiredAlerts(f) {
		if a.Attributes["severity"] != "warning" {
			t.Errorf("%s : staging n'alerte jamais en critical", a.Name)
		}
	}
	f, err = ParseSLO([]byte(withTier("preview")), "app-hello-preview")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(DesiredAlerts(f)); n != 0 {
		t.Errorf("préversion : aucune alerte attendue, %d obtenues", n)
	}
	only := strings.Replace(templateSLO, "tiers: [prod, staging]", "tiers: [prod]", 1)
	f, _ = ParseSLO([]byte(strings.Replace(only, "tier: prod", "tier: staging", 1)), "app-hello-staging")
	if n := len(DesiredAlerts(f)); n != 0 {
		t.Errorf("tier non listé dans alerting.tiers : %d alertes", n)
	}
}

func TestHashStable(t *testing.T) {
	f, _ := ParseSLO([]byte(templateSLO), "app-hello-prod")
	a, b := DesiredAlerts(f), DesiredAlerts(f)
	if a[0].Hash != b[0].Hash {
		t.Fatal("empreinte instable")
	}
	g, _ := ParseSLO([]byte(strings.Replace(templateSLO, "threshold_ms: 400", "threshold_ms: 500", 1)), "app-hello-prod")
	for i, x := range DesiredAlerts(g) {
		if x.Name == "slo-hello-prod-latence-fast" && x.Hash == a[i].Hash {
			t.Error("l'empreinte doit changer avec le seuil")
		}
	}
}

func TestRejects(t *testing.T) {
	cases := map[string]struct{ yaml, ns string }{
		"autre namespace":        {templateSLO, "app-autre-prod"},
		"service usurpé":         {strings.Replace(templateSLO, "service: hello", "service: autre", 1), "app-hello-prod"},
		"injection dans route":   {strings.Replace(templateSLO, `"/readyz"`, `"/x') OR 1=1 --"`, 1), "app-hello-prod"},
		"injection dans service": {strings.Replace(templateSLO, "service: hello", "service: \"h' OR '1\"", 1), "app-h' OR '1-prod"},
		"good arbitraire":        {strings.Replace(templateSLO, "status_code < 500", "status_code < 400", 1), "app-hello-prod"},
		"champ inconnu":          {templateSLO + "extra: 1\n", "app-hello-prod"},
		"target hors bornes":     {strings.Replace(templateSLO, "target: 99.5", "target: 100", 1), "app-hello-prod"},
		"seuil inatteignable":    {strings.Replace(templateSLO, "target: 95", "target: 90", 1), "app-hello-prod"},
		"fenêtre invalide":       {strings.Replace(templateSLO, "window: 5m", "window: 5s", 1), "app-hello-prod"},
		"tier non alertable":     {strings.Replace(templateSLO, "tiers: [prod, staging]", "tiers: [preview]", 1), "app-hello-prod"},
		"mauvais kind":           {strings.Replace(templateSLO, "kind: ServiceLevelObjectives", "kind: Autre", 1), "app-hello-prod"},
		"objectif en double":     {strings.Replace(templateSLO, "name: latence", "name: disponibilite", 1), "app-hello-prod"},
		"fichier vide":           {"", "app-hello-prod"},
	}
	for name, c := range cases {
		if _, err := ParseSLO([]byte(c.yaml), c.ns); err == nil {
			t.Errorf("%s : erreur attendue", name)
		}
	}
}
