// Package appconn adapts depman credentials/store into portable connectors.
// This is the only layer that knows about env-scoped credentials and AES secrets.
package appconn

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"depman/internal/connector"
	"depman/internal/connector/git"
	"depman/internal/connector/rancher"
	"depman/internal/connector/registry"
	connssh "depman/internal/connector/ssh"
	"depman/internal/model"
)

// Extra is the optional JSON blob on credentials.extra.
type Extra struct {
	Provider  string `json:"provider,omitempty"`  // e.g. rancher
	ClusterID string `json:"clusterId,omitempty"`
	Repo      string `json:"repo,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Auth      string `json:"auth,omitempty"` // https | ssh
	Project   string `json:"project,omitempty"`
	Insecure  *bool  `json:"insecure,omitempty"` // TLS skip verify; default true for registry/rancher
}

func ParseExtra(raw string) Extra {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Extra{}
	}
	var e Extra
	_ = json.Unmarshal([]byte(raw), &e)
	return e
}

func (e Extra) tlsInsecure(defaultInsecure bool) bool {
	if e.Insecure != nil {
		return *e.Insecure
	}
	return defaultInsecure
}

// baseURL builds http(s)://host[:port] from credential host/port.
// If host already contains a scheme or :port, it is respected (no double port).
func baseURL(host string, port int, schemeDefault string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		return strings.TrimRight(host, "/")
	}

	h := host
	p := port
	if ah, ap, err := net.SplitHostPort(host); err == nil {
		h = ah
		if p <= 0 {
			if pi, convErr := strconv.Atoi(ap); convErr == nil {
				p = pi
			}
		}
	}

	scheme := schemeDefault
	switch p {
	case 443:
		scheme = "https"
	case 80:
		scheme = "http"
	}
	if p > 0 && p != 80 && p != 443 {
		return fmt.Sprintf("%s://%s:%d", scheme, h, p)
	}
	return scheme + "://" + h
}

// SSHConfig maps a credential + plaintext secret to connector/ssh.Config.
func SSHConfig(c model.Credential, secret string) connssh.Config {
	return connssh.ConfigFromSecret(c.Host, c.Port, c.Username, secret)
}

// RegistryConfig maps harbor (or any OCI) credential.
func RegistryConfig(c model.Credential, secret string) registry.Config {
	ex := ParseExtra(c.Extra)
	method := connector.AuthBasic
	if c.Username == "" && secret != "" {
		method = connector.AuthBearer
	}
	return registry.Config{
		Endpoint: connector.Endpoint{
			BaseURL:  baseURL(c.Host, c.Port, "https"),
			Timeout:  30 * time.Second,
			Insecure: ex.tlsInsecure(true),
		},
		Auth: connector.Auth{
			Username: c.Username,
			Secret:   secret,
			Method:   method,
		},
	}
}

// GitConfig maps a git credential.
func GitConfig(c model.Credential, secret string) git.Config {
	ex := ParseExtra(c.Extra)
	remote := strings.TrimSpace(c.Host)
	if remote != "" && !strings.Contains(remote, "://") && !strings.HasPrefix(remote, "git@") {
		base := strings.TrimRight(baseURL(c.Host, c.Port, "https"), "/")
		if ex.Repo != "" {
			remote = base + "/" + strings.TrimPrefix(ex.Repo, "/")
		} else {
			remote = base
		}
	}
	sshURL := strings.HasPrefix(remote, "git@") || strings.HasPrefix(strings.ToLower(remote), "ssh://")
	httpURL := strings.HasPrefix(strings.ToLower(remote), "http://") || strings.HasPrefix(strings.ToLower(remote), "https://")
	method := connector.AuthBasic
	switch {
	case sshURL || strings.EqualFold(ex.Auth, "ssh"):
		if strings.Contains(secret, "BEGIN") && strings.Contains(secret, "PRIVATE KEY") {
			method = connector.AuthSSHKey
		} else if secret != "" && !httpURL {
			method = connector.AuthSSHKey
		}
	case strings.Contains(secret, "BEGIN") && strings.Contains(secret, "PRIVATE KEY") && !httpURL:
		method = connector.AuthSSHKey
	}
	return git.Config{
		Endpoint: connector.Endpoint{BaseURL: remote, Timeout: 60 * time.Second},
		Auth: connector.Auth{
			Username: c.Username,
			Secret:   secret,
			Method:   method,
		},
		Branch: ex.Branch,
	}
}

// RancherConfig maps an api credential with provider=rancher (or default for type api).
func RancherConfig(c model.Credential, secret string) (rancher.Config, error) {
	ex := ParseExtra(c.Extra)
	provider := strings.ToLower(strings.TrimSpace(ex.Provider))
	if provider != "" && provider != "rancher" {
		return rancher.Config{}, fmt.Errorf("%w: unsupported api provider %q", connector.ErrInvalidConfig, ex.Provider)
	}
	return rancher.Config{
		Endpoint: connector.Endpoint{
			BaseURL:  baseURL(c.Host, c.Port, "https"),
			Timeout:  30 * time.Second,
			Insecure: ex.tlsInsecure(true),
		},
		Auth: connector.Auth{
			Username: c.Username,
			Secret:   secret,
			Method:   connector.AuthBearer,
		},
		ClusterID: ex.ClusterID,
	}, nil
}
