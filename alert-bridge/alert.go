package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Alert : représentation commune d'une alerte, quelle que soit sa source.
type Alert struct {
	Source   string `json:"source"`   // openobserve, uptimerobot, flux…
	Name     string `json:"alert"`    // nom stable de l'alerte (sert à l'empreinte)
	App      string `json:"app"`      // application concernée (vide : plateforme)
	Tier     string `json:"tier"`     // prod, staging, preview
	Severity string `json:"severity"` // critical, warning, info
	Status   string `json:"status"`   // firing ou resolved
	Message  string `json:"message"`
	URL      string `json:"url"` // lien vers le tableau de bord ou l'événement
}

var appName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,19}$`)

// Fingerprint : identifiant stable d'une alerte (indépendant du message et de l'horodatage).
func (a Alert) Fingerprint() string {
	h := sha256.Sum256([]byte(strings.Join([]string{a.Source, a.Name, a.App, a.Tier}, "\x00")))
	return hex.EncodeToString(h[:])[:12]
}

// Ignored : les préversions n'ouvrent pas de ticket (elles sont éphémères).
func (a Alert) Ignored() bool { return a.Tier == "preview" }

func (a Alert) Resolved() bool { return a.Status == "resolved" }

func (a Alert) Title() string {
	where := "plateforme"
	if a.App != "" {
		where = a.App
		if a.Tier != "" {
			where += "/" + a.Tier
		}
	}
	return fmt.Sprintf("[incident] %s : %s", where, a.Name)
}

func (a Alert) Labels() []string {
	sev := a.Severity
	if sev == "" {
		sev = "warning"
	}
	l := []string{"incident", "sev:" + sev, "fp:" + a.Fingerprint(), "source:" + a.Source}
	if a.App != "" {
		l = append(l, "app:"+a.App)
	}
	return l
}

func (a *Alert) normalize() error {
	a.Status = strings.ToLower(strings.TrimSpace(a.Status))
	if a.Status == "" || a.Status == "ok" || a.Status == "up" {
		if a.Status != "" {
			a.Status = "resolved"
		} else {
			a.Status = "firing"
		}
	}
	if a.Status == "down" || a.Status == "alerting" {
		a.Status = "firing"
	}
	if a.Status != "firing" && a.Status != "resolved" {
		return fmt.Errorf("statut inconnu : %q", a.Status)
	}
	if a.Name == "" || a.Source == "" {
		return errors.New("champs « source » et « alert » requis")
	}
	if a.App != "" && !appName.MatchString(a.App) {
		return fmt.Errorf("nom d'application invalide : %q", a.App)
	}
	if len(a.Message) > 4000 {
		a.Message = a.Message[:4000] + "…"
	}
	return nil
}

func parseGeneric(body []byte) (Alert, error) {
	var a Alert
	if err := json.Unmarshal(body, &a); err != nil {
		return a, fmt.Errorf("JSON invalide : %w", err)
	}
	return a, a.normalize()
}

// Événement du notification-controller de Flux (Provider type generic).
type fluxEvent struct {
	InvolvedObject struct {
		Kind      string `json:"kind"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"involvedObject"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Reason   string `json:"reason"`
}

// Les objets Flux des applications s'appellent app-<app>-<niveau> ou app-<app>-pr-<n>.
var fluxAppObject = regexp.MustCompile(`^app-([a-z][a-z0-9-]*?)-(prod|staging|pr-\d+)$`)

func parseFlux(body []byte) (Alert, error) {
	var e fluxEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return Alert{}, fmt.Errorf("JSON invalide : %w", err)
	}
	a := Alert{
		Source:   "flux",
		Name:     fmt.Sprintf("%s %s en échec", e.InvolvedObject.Kind, e.InvolvedObject.Name),
		Severity: "warning",
		Status:   "firing",
		Message:  fmt.Sprintf("%s (%s) : %s", e.Reason, e.InvolvedObject.Namespace, e.Message),
	}
	if e.Severity == "info" {
		a.Status = "resolved"
	}
	if m := fluxAppObject.FindStringSubmatch(e.InvolvedObject.Name); m != nil {
		a.App, a.Tier = m[1], m[2]
		if strings.HasPrefix(m[2], "pr-") {
			a.Tier = "preview"
		}
		if a.Tier == "prod" {
			a.Severity = "critical"
		}
	}
	return a, a.normalize()
}
