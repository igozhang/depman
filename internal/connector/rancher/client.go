// Package rancher implements k8sapi.Client against the Rancher HTTP API.
package rancher

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"depman/internal/connector"
	"depman/internal/connector/k8sapi"
)

// Config targets a Rancher server.
type Config struct {
	Endpoint  connector.Endpoint // BaseURL e.g. https://rancher.example
	Auth      connector.Auth     // typically AuthBearer with API key/token
	ClusterID string             // optional default cluster
}

// Client is a Rancher-backed k8sapi.Client.
type Client struct {
	Cfg Config
	hc  *http.Client
}

func New(cfg Config) *Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Endpoint.Insecure, //nolint:gosec // ops tool / internal CA
		},
	}
	return &Client{
		Cfg: cfg,
		hc: &http.Client{
			Timeout:   cfg.Endpoint.TimeoutOr(30 * time.Second),
			Transport: tr,
		},
	}
}

var _ k8sapi.Client = (*Client)(nil)

func (c *Client) validate() error {
	if strings.TrimSpace(c.Cfg.Endpoint.BaseURL) == "" {
		return fmt.Errorf("%w: rancher URL empty", connector.ErrInvalidConfig)
	}
	if c.Cfg.Auth.Method == "" {
		c.Cfg.Auth.Method = connector.AuthBearer
	}
	if strings.TrimSpace(c.Cfg.Auth.Secret) == "" {
		return fmt.Errorf("%w: rancher API token empty", connector.ErrInvalidConfig)
	}
	return nil
}

func (c *Client) base() string {
	return strings.TrimRight(strings.TrimSpace(c.Cfg.Endpoint.BaseURL), "/")
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(b)
	}
	u := c.base() + path
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token := strings.TrimSpace(c.Cfg.Auth.Secret)
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := c.hc.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", connector.ErrUnavailable, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return data, res.StatusCode, fmt.Errorf("%w: status %d", connector.ErrAuth, res.StatusCode)
	}
	if res.StatusCode >= 400 {
		return data, res.StatusCode, fmt.Errorf("%w: status %d: %s", connector.ErrUnavailable, res.StatusCode, truncate(string(data), 300))
	}
	return data, res.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func (c *Client) clusterID(id string) string {
	if strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id)
	}
	return strings.TrimSpace(c.Cfg.ClusterID)
}

func (c *Client) Ping(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(15 * time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, _, err := c.do(ctx, http.MethodGet, "/v3/clusters?limit=1", nil)
	return err
}

type normanList struct {
	Data []json.RawMessage `json:"data"`
}

type clusterDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

func (c *Client) ListClusters(ctx context.Context) ([]k8sapi.ClusterInfo, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	data, _, err := c.do(ctx, http.MethodGet, "/v3/clusters", nil)
	if err != nil {
		return nil, err
	}
	var list normanList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	out := make([]k8sapi.ClusterInfo, 0, len(list.Data))
	for _, raw := range list.Data {
		var d clusterDTO
		if json.Unmarshal(raw, &d) != nil || d.ID == "" {
			continue
		}
		out = append(out, k8sapi.ClusterInfo{ID: d.ID, Name: d.Name, State: d.State})
	}
	return out, nil
}

type steveList struct {
	Data []steveObj `json:"data"`
}

type steveObj struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Links struct {
		Self string `json:"self"`
	} `json:"links"`
	Metadata struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec   json.RawMessage `json:"spec"`
	Status json.RawMessage `json:"status"`
}

func (c *Client) ListNamespaces(ctx context.Context, clusterID string) ([]k8sapi.NamespaceInfo, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	clusterID = c.clusterID(clusterID)
	if clusterID == "" {
		return nil, fmt.Errorf("%w: clusterID empty", connector.ErrInvalidConfig)
	}
	path := "/k8s/clusters/" + url.PathEscape(clusterID) + "/v1/namespaces"
	data, _, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var list steveList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	out := make([]k8sapi.NamespaceInfo, 0, len(list.Data))
	for _, o := range list.Data {
		name := o.Metadata.Name
		if name == "" {
			name = o.ID
		}
		status := ""
		var st struct {
			Phase string `json:"phase"`
		}
		_ = json.Unmarshal(o.Status, &st)
		status = st.Phase
		out = append(out, k8sapi.NamespaceInfo{Name: name, Status: status})
	}
	return out, nil
}

func (c *Client) ListWorkloads(ctx context.Context, clusterID, namespace string) ([]k8sapi.WorkloadInfo, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	clusterID = c.clusterID(clusterID)
	if clusterID == "" || namespace == "" {
		return nil, fmt.Errorf("%w: clusterID and namespace required", connector.ErrInvalidConfig)
	}
	kinds := []struct {
		api  string
		kind string
	}{
		{"apps.deployments", "Deployment"},
		{"apps.statefulsets", "StatefulSet"},
		{"apps.daemonsets", "DaemonSet"},
	}
	var out []k8sapi.WorkloadInfo
	for _, k := range kinds {
		path := fmt.Sprintf("/k8s/clusters/%s/v1/%s?namespace=%s",
			url.PathEscape(clusterID), k.api, url.QueryEscape(namespace))
		data, _, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		var list steveList
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		for _, o := range list.Data {
			out = append(out, steveToWorkload(o, k.kind))
		}
	}
	return out, nil
}

func steveToWorkload(o steveObj, kind string) k8sapi.WorkloadInfo {
	name := o.Metadata.Name
	ns := o.Metadata.Namespace
	var replicas, ready int32
	var image string
	var spec struct {
		Replicas *int32 `json:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	}
	_ = json.Unmarshal(o.Spec, &spec)
	if spec.Replicas != nil {
		replicas = *spec.Replicas
	}
	if len(spec.Template.Spec.Containers) > 0 {
		image = spec.Template.Spec.Containers[0].Image
	}
	var status struct {
		ReadyReplicas     int32 `json:"readyReplicas"`
		NumberReady       int32 `json:"numberReady"`
		AvailableReplicas int32 `json:"availableReplicas"`
	}
	_ = json.Unmarshal(o.Status, &status)
	ready = status.ReadyReplicas
	if ready == 0 && status.NumberReady > 0 {
		ready = status.NumberReady
	}
	if ready == 0 && status.AvailableReplicas > 0 {
		ready = status.AvailableReplicas
	}
	return k8sapi.WorkloadInfo{
		Ref:      k8sapi.ObjectRef{Namespace: ns, Name: name, Kind: kind},
		Replicas: replicas,
		Ready:    ready,
		Image:    image,
	}
}

func workloadAPI(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "deployment", "deployments", "apps.deployment", "apps.deployments":
		return "apps.deployments", nil
	case "statefulset", "statefulsets", "apps.statefulset", "apps.statefulsets":
		return "apps.statefulsets", nil
	case "daemonset", "daemonsets", "apps.daemonset", "apps.daemonsets":
		return "apps.daemonsets", nil
	default:
		return "", fmt.Errorf("%w: unsupported kind %q", connector.ErrInvalidConfig, kind)
	}
}

func (c *Client) getWorkload(ctx context.Context, clusterID string, ref k8sapi.ObjectRef) (map[string]any, string, error) {
	api, err := workloadAPI(ref.Kind)
	if err != nil {
		return nil, "", err
	}
	if ref.Namespace == "" || ref.Name == "" {
		return nil, "", fmt.Errorf("%w: namespace and name required", connector.ErrInvalidConfig)
	}
	path := fmt.Sprintf("/k8s/clusters/%s/v1/%s/%s/%s",
		url.PathEscape(clusterID), api, url.PathEscape(ref.Namespace), url.PathEscape(ref.Name))
	data, _, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, "", err
	}
	return obj, path, nil
}

func (c *Client) Scale(ctx context.Context, clusterID string, spec k8sapi.ScaleSpec) error {
	if err := c.validate(); err != nil {
		return err
	}
	clusterID = c.clusterID(clusterID)
	if clusterID == "" || spec.Ref.Name == "" {
		return fmt.Errorf("%w: clusterID and workload name required", connector.ErrInvalidConfig)
	}
	if strings.EqualFold(spec.Ref.Kind, "DaemonSet") || strings.EqualFold(spec.Ref.Kind, "daemonsets") {
		return fmt.Errorf("%w: DaemonSet cannot be scaled by replicas", connector.ErrInvalidConfig)
	}
	obj, path, err := c.getWorkload(ctx, clusterID, spec.Ref)
	if err != nil {
		return err
	}
	sp, _ := obj["spec"].(map[string]any)
	if sp == nil {
		sp = map[string]any{}
		obj["spec"] = sp
	}
	sp["replicas"] = spec.Replicas
	_, _, err = c.do(ctx, http.MethodPut, path, obj)
	return err
}

func (c *Client) Restart(ctx context.Context, clusterID string, ref k8sapi.ObjectRef) error {
	if err := c.validate(); err != nil {
		return err
	}
	clusterID = c.clusterID(clusterID)
	if clusterID == "" || ref.Name == "" {
		return fmt.Errorf("%w: clusterID and workload name required", connector.ErrInvalidConfig)
	}
	obj, path, err := c.getWorkload(ctx, clusterID, ref)
	if err != nil {
		return err
	}
	md, _ := obj["metadata"].(map[string]any)
	if md == nil {
		md = map[string]any{}
		obj["metadata"] = md
	}
	ann := map[string]any{}
	switch existing := md["annotations"].(type) {
	case map[string]any:
		for k, v := range existing {
			ann[k] = v
		}
	case map[string]string:
		for k, v := range existing {
			ann[k] = v
		}
	}
	ann["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
	md["annotations"] = ann
	_, _, err = c.do(ctx, http.MethodPut, path, obj)
	return err
}
