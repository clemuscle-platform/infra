package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

type alertAPI interface {
	EnsureFolder(ctx context.Context) (string, error)
	EnsureNotifier(ctx context.Context, bridgeURL, token string) error
	ListAlerts(ctx context.Context, folderID string) ([]existingAlert, error)
	CreateAlert(ctx context.Context, folderID string, a AlertSpec) error
	UpdateAlert(ctx context.Context, folderID, id string, a AlertSpec) error
	DeleteAlert(ctx context.Context, id string) error
}

type result struct {
	Created, Updated, Deleted, Unchanged int
	Errors                               []string
}

// plan : alertes voulues à partir des fichiers SLO. Un fichier invalide est signalé et son
// préfixe d'alertes est « protégé » : on ne supprime pas les alertes existantes d'une
// application parce que son fichier est cassé.
func plan(sources []sloSource) (desired []AlertSpec, protected []string, errs []string) {
	seen := map[string]string{}
	for _, s := range sources {
		f, err := ParseSLO(s.Data, s.Namespace)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s/%s : %v", s.Namespace, s.Name, err))
			protected = append(protected, "slo-"+strings.TrimPrefix(s.Namespace, "app-")+"-")
			continue
		}
		for _, a := range DesiredAlerts(f) {
			if prev, dup := seen[a.Name]; dup {
				errs = append(errs, fmt.Sprintf("%s/%s : alerte %s déjà déclarée par %s", s.Namespace, s.Name, a.Name, prev))
				continue
			}
			seen[a.Name] = s.Namespace + "/" + s.Name
			desired = append(desired, a)
		}
	}
	sort.Slice(desired, func(i, j int) bool { return desired[i].Name < desired[j].Name })
	return desired, protected, errs
}

// reconcile applique le plan : création, mise à jour si l'empreinte a changé, suppression des
// alertes gérées par slo-sync qui ne sont plus déclarées. Une alerte sans le tag
// managed-by:slo-sync n'est jamais modifiée ni supprimée.
func reconcile(ctx context.Context, api alertAPI, folderID string, desired []AlertSpec, protected []string, dryRun bool, log *slog.Logger) (result, error) {
	var r result
	existing, err := api.ListAlerts(ctx, folderID)
	if err != nil {
		return r, fmt.Errorf("liste des alertes : %w", err)
	}
	byName := map[string]existingAlert{}
	for _, e := range existing {
		byName[e.Name] = e
	}
	want := map[string]bool{}
	for _, a := range desired {
		want[a.Name] = true
		e, ok := byName[a.Name]
		switch {
		case !ok:
			log.Info("création", "alerte", a.Name, "dry_run", dryRun)
			if !dryRun {
				if err := api.CreateAlert(ctx, folderID, a); err != nil {
					r.Errors = append(r.Errors, fmt.Sprintf("création %s : %v", a.Name, err))
					continue
				}
			}
			r.Created++
		case !e.managed():
			r.Errors = append(r.Errors, fmt.Sprintf("%s existe déjà sans le tag %s : laissée intacte", a.Name, managedTag))
		case e.hash() != a.Hash:
			log.Info("mise à jour", "alerte", a.Name, "dry_run", dryRun)
			if !dryRun {
				if err := api.UpdateAlert(ctx, folderID, e.ID, a); err != nil {
					r.Errors = append(r.Errors, fmt.Sprintf("mise à jour %s : %v", a.Name, err))
					continue
				}
			}
			r.Updated++
		default:
			r.Unchanged++
		}
	}
	for _, e := range existing {
		if want[e.Name] || !e.managed() || isProtected(e.Name, protected) {
			continue
		}
		log.Info("suppression", "alerte", e.Name, "dry_run", dryRun)
		if !dryRun {
			if err := api.DeleteAlert(ctx, e.ID); err != nil {
				r.Errors = append(r.Errors, fmt.Sprintf("suppression %s : %v", e.Name, err))
				continue
			}
		}
		r.Deleted++
	}
	return r, nil
}

func isProtected(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
