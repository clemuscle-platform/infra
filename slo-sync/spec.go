package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Fichier deploy/slo.yaml d'une application (kind ServiceLevelObjectives), livré par Flux
// dans une ConfigMap labellisée platform.io/slo=true du namespace de l'application.
type SLOFile struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Service    string      `yaml:"service"`
	Tier       string      `yaml:"tier"`
	Objectives []Objective `yaml:"objectives"`
	Alerting   Alerting    `yaml:"alerting"`
}

type Objective struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Target      float64   `yaml:"target"`
	ThresholdMS int       `yaml:"threshold_ms"`
	Indicator   Indicator `yaml:"indicator"`
}

type Indicator struct {
	Type          string   `yaml:"type"`
	Good          string   `yaml:"good"`
	ExcludeRoutes []string `yaml:"exclude_routes"`
}

type Alerting struct {
	FastBurn Burn     `yaml:"fast_burn"`
	SlowBurn Burn     `yaml:"slow_burn"`
	Tiers    []string `yaml:"tiers"`
}

type Burn struct {
	Window   string  `yaml:"window"`
	BurnRate float64 `yaml:"burn_rate"`
}

const (
	// En dessous de ce volume de requêtes sur la fenêtre, aucune alerte : un taux calculé
	// sur trois requêtes n'a pas de sens.
	minRequests = 20
	// Seule forme acceptée pour indicator.good d'un error-rate : le SQL est généré par slo-sync,
	// jamais recopié depuis le fichier de l'application.
	errorRateGood = "http.response.status_code < 500"
)

var (
	appName   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,19}$`)
	objName   = regexp.MustCompile(`^[a-z][a-z0-9-]{1,30}$`)
	routeRe   = regexp.MustCompile(`^/[A-Za-z0-9._~/{}:-]{0,200}$`)
	windowRe  = regexp.MustCompile(`^([1-9][0-9]{0,3})(m|h)$`)
	tiers     = map[string]bool{"preview": true, "staging": true, "prod": true}
	alertable = map[string]bool{"staging": true, "prod": true}
)

// ParseSLO lit et valide le fichier d'une application. Le namespace est celui de la ConfigMap :
// il doit correspondre exactement à app-<service>-<tier>, pour qu'une application ne puisse
// pas déclarer d'alertes au nom d'une autre.
func ParseSLO(data []byte, namespace string) (SLOFile, error) {
	var f SLOFile
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return f, fmt.Errorf("YAML invalide : %w", err)
	}
	if f.APIVersion != "platform.io/v1" || f.Kind != "ServiceLevelObjectives" {
		return f, fmt.Errorf("apiVersion/kind attendus : platform.io/v1 ServiceLevelObjectives (reçu %q %q)", f.APIVersion, f.Kind)
	}
	if !appName.MatchString(f.Service) {
		return f, fmt.Errorf("service invalide : %q", f.Service)
	}
	if !tiers[f.Tier] {
		return f, fmt.Errorf("tier invalide : %q", f.Tier)
	}
	if want := "app-" + f.Service + "-" + f.Tier; namespace != want {
		return f, fmt.Errorf("namespace %q ne correspond pas au service et au tier déclarés (attendu %q)", namespace, want)
	}
	if len(f.Objectives) == 0 {
		return f, errors.New("aucun objectif")
	}
	seen := map[string]bool{}
	for _, o := range f.Objectives {
		if !objName.MatchString(o.Name) {
			return f, fmt.Errorf("nom d'objectif invalide : %q", o.Name)
		}
		if seen[o.Name] {
			return f, fmt.Errorf("objectif en double : %q", o.Name)
		}
		seen[o.Name] = true
		if o.Target <= 50 || o.Target >= 100 {
			return f, fmt.Errorf("%s : target doit être dans ]50, 100[ (reçu %v)", o.Name, o.Target)
		}
		switch o.Indicator.Type {
		case "error-rate":
			if o.Indicator.Good != "" && o.Indicator.Good != errorRateGood {
				return f, fmt.Errorf("%s : seul good: '%s' est pris en charge", o.Name, errorRateGood)
			}
		case "latency":
			if o.ThresholdMS < 1 || o.ThresholdMS > 60000 {
				return f, fmt.Errorf("%s : threshold_ms doit être entre 1 et 60000", o.Name)
			}
			for _, r := range o.Indicator.ExcludeRoutes {
				if !routeRe.MatchString(r) {
					return f, fmt.Errorf("%s : route exclue invalide : %q", o.Name, r)
				}
			}
		default:
			return f, fmt.Errorf("%s : type d'indicateur inconnu : %q", o.Name, o.Indicator.Type)
		}
	}
	for _, b := range []struct {
		label string
		burn  Burn
	}{{"fast_burn", f.Alerting.FastBurn}, {"slow_burn", f.Alerting.SlowBurn}} {
		if _, err := windowMinutes(b.burn.Window); err != nil {
			return f, fmt.Errorf("alerting.%s : %w", b.label, err)
		}
		if b.burn.BurnRate < 1 || b.burn.BurnRate > 1000 {
			return f, fmt.Errorf("alerting.%s : burn_rate doit être entre 1 et 1000", b.label)
		}
		for _, o := range f.Objectives {
			if ratio(o, b.burn) >= 1 {
				return f, fmt.Errorf("alerting.%s : avec %s (target %v), le seuil dépasse 100 %% : l'alerte ne pourrait jamais se déclencher", b.label, o.Name, o.Target)
			}
		}
	}
	for _, t := range f.Alerting.Tiers {
		if !alertable[t] {
			return f, fmt.Errorf("alerting.tiers : %q n'est pas alertable (staging ou prod)", t)
		}
	}
	return f, nil
}

func windowMinutes(w string) (int, error) {
	m := windowRe.FindStringSubmatch(w)
	if m == nil {
		return 0, fmt.Errorf("fenêtre invalide : %q (ex. 5m, 1h)", w)
	}
	n, _ := strconv.Atoi(m[1])
	if m[2] == "h" {
		n *= 60
	}
	if n > 1440 {
		return 0, fmt.Errorf("fenêtre trop longue : %q (24 h maximum)", w)
	}
	return n, nil
}

// ratio : part maximale de requêtes « mauvaises » tolérée sur la fenêtre avant alerte
// (burn rate × budget d'erreur).
func ratio(o Objective, b Burn) float64 {
	return b.BurnRate * (1 - o.Target/100)
}

// AlertSpec : une alerte OpenObserve voulue. Hash résume tout ce qui est envoyé à l'API,
// pour ne mettre à jour que ce qui a changé.
type AlertSpec struct {
	Name       string
	App        string
	Tier       string
	SQL        string
	PeriodMin  int
	EveryMin   int
	SilenceMin int
	Attributes map[string]string
	Hash       string
}

// DesiredAlerts : deux alertes (fenêtre rapide et lente) par objectif, pour les seuls tiers alertables.
func DesiredAlerts(f SLOFile) []AlertSpec {
	enabled := false
	for _, t := range f.Alerting.Tiers {
		if t == f.Tier {
			enabled = true
		}
	}
	if !enabled || f.Tier == "preview" {
		return nil
	}
	var out []AlertSpec
	for _, o := range f.Objectives {
		for _, w := range []struct {
			speed string
			burn  Burn
			every int
			quiet int
		}{
			{"fast", f.Alerting.FastBurn, 1, 15},
			{"slow", f.Alerting.SlowBurn, 5, 60},
		} {
			period, _ := windowMinutes(w.burn.Window)
			severity := "warning"
			if f.Tier == "prod" && w.speed == "fast" {
				severity = "critical"
			}
			a := AlertSpec{
				Name:       fmt.Sprintf("slo-%s-%s-%s-%s", f.Service, f.Tier, o.Name, w.speed),
				App:        f.Service,
				Tier:       f.Tier,
				SQL:        buildSQL(f, o, w.burn),
				PeriodMin:  period,
				EveryMin:   w.every,
				SilenceMin: w.quiet,
				Attributes: map[string]string{
					"app": f.Service, "tier": f.Tier, "severity": severity,
					"objective": o.Name, "window": w.burn.Window,
				},
			}
			a.Hash = hashOf(a)
			out = append(out, a)
		}
	}
	return out
}

// buildSQL : la requête ne renvoie une ligne que si la part de requêtes mauvaises dépasse
// le seuil (HAVING), avec un volume minimal. Champs vérifiés sur OpenObserve 1.1.2 :
// http_response_status_code est stocké en texte, duration en microsecondes, span_kind '2' = SERVER.
// Toutes les valeurs interpolées ont été validées par ParseSLO (pas de guillemet possible).
func buildSQL(f SLOFile, o Objective, b Burn) string {
	ns := "app-" + f.Service + "-" + f.Tier
	where := fmt.Sprintf(`service_service_namespace = '%s' AND span_kind = '2'`, ns)
	var bad string
	switch o.Indicator.Type {
	case "error-rate":
		bad = "CAST(http_response_status_code AS INT) >= 500"
	case "latency":
		bad = fmt.Sprintf("duration > %d", o.ThresholdMS*1000)
		if len(o.Indicator.ExcludeRoutes) > 0 {
			q := make([]string, len(o.Indicator.ExcludeRoutes))
			for i, r := range o.Indicator.ExcludeRoutes {
				q[i] = "'" + r + "'"
			}
			where += fmt.Sprintf(" AND http_route NOT IN (%s)", strings.Join(q, ", "))
		}
	}
	badSum := fmt.Sprintf("sum(CASE WHEN %s THEN 1 ELSE 0 END)", bad)
	return fmt.Sprintf(`SELECT count(*) AS total, %s AS bad FROM "%s" WHERE %s HAVING count(*) >= %d AND %s > %s * count(*)`,
		badSum, tracesStream, where, minRequests, badSum, strconv.FormatFloat(ratio(o, b), 'f', 6, 64))
}

func hashOf(a AlertSpec) string {
	b, _ := json.Marshal([]any{a.Name, a.SQL, a.PeriodMin, a.EveryMin, a.SilenceMin, a.Attributes, templateBody})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:12]
}
