package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	saDir       = "/var/run/secrets/kubernetes.io/serviceaccount"
	sloLabel    = "platform.io/slo=true"
	sloFileName = "slo.yaml"
)

type sloSource struct {
	Namespace, Name string
	Data            []byte
}

type kubeClient struct {
	base  string
	token string
	http  *http.Client
}

// newInClusterKube : client minimal (lecture seule) à partir du ServiceAccount monté.
func newInClusterKube() (*kubeClient, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, errors.New("hors cluster : KUBERNETES_SERVICE_HOST absent")
	}
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return nil, fmt.Errorf("jeton du ServiceAccount : %w", err)
	}
	ca, err := os.ReadFile(saDir + "/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("CA du cluster : %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("CA du cluster illisible")
	}
	return &kubeClient{
		base:  "https://" + net.JoinHostPort(host, port),
		token: string(token),
		http: &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		}},
	}, nil
}

// ListSLO : toutes les ConfigMaps labellisées platform.io/slo=true, tous namespaces confondus.
func (k *kubeClient) ListSLO(ctx context.Context) ([]sloSource, error) {
	u := k.base + "/api/v1/configmaps?" + url.Values{"labelSelector": {sloLabel}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	resp, err := k.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("liste des ConfigMaps : HTTP %d : %s", resp.StatusCode, raw)
	}
	return decodeConfigMaps(raw)
}

func decodeConfigMaps(raw []byte) ([]sloSource, error) {
	var list struct {
		Metadata struct {
			Continue string `json:"continue"`
		} `json:"metadata"`
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Data map[string]string `json:"data"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("liste des ConfigMaps illisible : %w", err)
	}
	if list.Metadata.Continue != "" {
		return nil, errors.New("liste des ConfigMaps paginée : non géré, arrêt par prudence")
	}
	out := make([]sloSource, 0, len(list.Items))
	for _, it := range list.Items {
		out = append(out, sloSource{Namespace: it.Metadata.Namespace, Name: it.Metadata.Name, Data: []byte(it.Data[sloFileName])})
	}
	return out, nil
}
