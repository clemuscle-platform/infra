package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	folderName = "slo"
	// Modèle et destination partagés par toutes les alertes SLO (une seule cible : alert-bridge).
	notifierName = "platform-alert-bridge"
	managedTag   = "managed-by:slo-sync"
	hashTagPfx   = "slo-hash:"
	// Corps envoyé à alert-bridge (/hooks/generic). Variables vérifiées sur OpenObserve 1.1.2 :
	// {alert_name}, {alert_status} (firing | resolved) et les context_attributes de l'alerte.
	templateBody = `{"source":"openobserve","alert":"{alert_name}","app":"{app}","tier":"{tier}","severity":"{severity}","status":"{alert_status}","message":"SLO {objective} : budget d'erreur consommé trop vite (fenêtre {window})"}`
)

var errNotFound = errors.New("introuvable")

type o2Client struct {
	base, org, user, pass string
	http                  *http.Client
}

func (c *o2Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	u := strings.TrimRight(c.base, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s : HTTP %d : %s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%s %s : réponse illisible : %w", method, path, err)
		}
	}
	return nil
}

func (c *o2Client) v1(p string) string { return "/api/" + url.PathEscape(c.org) + p }
func (c *o2Client) v2(p string) string { return "/api/v2/" + url.PathEscape(c.org) + p }

// EnsureFolder renvoie l'identifiant du dossier d'alertes « slo », créé au besoin.
func (c *o2Client) EnsureFolder(ctx context.Context) (string, error) {
	var f struct {
		FolderID string `json:"folderId"`
	}
	err := c.do(ctx, http.MethodGet, c.v2("/folders/alerts/name/"+folderName), nil, nil, &f)
	if errors.Is(err, errNotFound) {
		err = c.do(ctx, http.MethodPost, c.v2("/folders/alerts"), nil,
			map[string]string{"name": folderName, "description": "Alertes SLO générées par slo-sync (ne pas modifier à la main)"}, &f)
	}
	if err != nil {
		return "", err
	}
	if f.FolderID == "" {
		return "", errors.New("dossier « slo » sans identifiant")
	}
	return f.FolderID, nil
}

// upsert : PUT si l'objet existe, sinon POST. Idempotent, rejoué à chaque passage
// (le jeton d'alert-bridge peut avoir changé).
func (c *o2Client) upsert(ctx context.Context, collection, name string, obj any) error {
	err := c.do(ctx, http.MethodGet, c.v1(collection+"/"+url.PathEscape(name)), nil, nil, nil)
	switch {
	case errors.Is(err, errNotFound):
		return c.do(ctx, http.MethodPost, c.v1(collection), nil, obj, nil)
	case err != nil:
		return err
	default:
		return c.do(ctx, http.MethodPut, c.v1(collection+"/"+url.PathEscape(name)), nil, obj, nil)
	}
}

func (c *o2Client) EnsureNotifier(ctx context.Context, bridgeURL, token string) error {
	tpl := map[string]any{"name": notifierName, "type": "http", "title": "SLO", "body": templateBody}
	if err := c.upsert(ctx, "/alerts/templates", notifierName, tpl); err != nil {
		return fmt.Errorf("modèle : %w", err)
	}
	dst := map[string]any{
		"name": notifierName, "type": "http", "url": bridgeURL, "method": "post",
		"template": notifierName, "skip_tls_verify": false,
		"headers": map[string]string{"X-Platform-Token": token},
	}
	if err := c.upsert(ctx, "/alerts/destinations", notifierName, dst); err != nil {
		return fmt.Errorf("destination : %w", err)
	}
	return nil
}

type existingAlert struct {
	ID   string   `json:"alert_id"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (a existingAlert) managed() bool {
	for _, t := range a.Tags {
		if t == managedTag {
			return true
		}
	}
	return false
}

func (a existingAlert) hash() string {
	for _, t := range a.Tags {
		if strings.HasPrefix(t, hashTagPfx) {
			return strings.TrimPrefix(t, hashTagPfx)
		}
	}
	return ""
}

func (c *o2Client) ListAlerts(ctx context.Context, folderID string) ([]existingAlert, error) {
	var out struct {
		List []existingAlert `json:"list"`
	}
	q := url.Values{"folder": {folderID}, "page_size": {"1000"}}
	if err := c.do(ctx, http.MethodGet, c.v2("/alerts"), q, nil, &out); err != nil {
		return nil, err
	}
	if len(out.List) >= 1000 {
		return nil, errors.New("plus de 1000 alertes dans le dossier : pagination non gérée, arrêt par prudence")
	}
	return out.List, nil
}

func alertBody(a AlertSpec) map[string]any {
	return map[string]any{
		"name":        a.Name,
		"stream_type": "traces", "stream_name": tracesStream, "is_real_time": false, "enabled": true,
		"query_condition": map[string]any{"type": "sql", "sql": a.SQL},
		"trigger_condition": map[string]any{
			"period": a.PeriodMin, "operator": ">=", "threshold": 1,
			"frequency": a.EveryMin, "frequency_type": "minutes", "silence": a.SilenceMin,
		},
		"destinations":       []string{notifierName},
		"notify_on_recovery": true,
		"context_attributes": a.Attributes,
		"tags":               []string{managedTag, hashTagPfx + a.Hash, "app:" + a.App, "tier:" + a.Tier},
	}
}

func (c *o2Client) CreateAlert(ctx context.Context, folderID string, a AlertSpec) error {
	return c.do(ctx, http.MethodPost, c.v2("/alerts"), url.Values{"folder": {folderID}}, alertBody(a), nil)
}

func (c *o2Client) UpdateAlert(ctx context.Context, folderID, id string, a AlertSpec) error {
	return c.do(ctx, http.MethodPut, c.v2("/alerts/"+url.PathEscape(id)), url.Values{"folder": {folderID}}, alertBody(a), nil)
}

func (c *o2Client) DeleteAlert(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, c.v2("/alerts/"+url.PathEscape(id)), nil, nil, nil)
}
