package appconn

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"depman/internal/connector"
	"depman/internal/connector/git"
	"depman/internal/connector/rancher"
	"depman/internal/connector/registry"
	connssh "depman/internal/connector/ssh"
	"depman/internal/model"
)

// PingResult is a normalized connectivity probe response.
type PingResult struct {
	Kind    connector.Kind `json:"kind"`
	OK      bool           `json:"ok"`
	Message string         `json:"message,omitempty"`
}

// PingCredential builds the right connector and probes it.
func PingCredential(ctx context.Context, c model.Credential, secret string) PingResult {
	switch strings.ToLower(strings.TrimSpace(c.Type)) {
	case "ssh":
		err := connssh.New(SSHConfig(c, secret)).Ping(ctx)
		return wrapPing(connector.KindSSH, err)
	case "harbor":
		err := registry.New(RegistryConfig(c, secret)).Ping(ctx)
		return wrapPing(connector.KindRegistry, err)
	case "git":
		cfg := GitConfig(c, secret)
		if gitHostOnly(cfg.Endpoint.BaseURL) {
			err := pingTCPHost(ctx, cfg.Endpoint.BaseURL, c.Port)
			return wrapPing(connector.KindGit, err)
		}
		err := git.New(cfg).Ping(ctx)
		return wrapPing(connector.KindGit, err)
	case "api":
		cfg, err := RancherConfig(c, secret)
		if err != nil {
			return PingResult{Kind: connector.KindCluster, OK: false, Message: err.Error()}
		}
		err = rancher.New(cfg).Ping(ctx)
		return wrapPing(connector.KindCluster, err)
	default:
		return PingResult{OK: false, Message: fmt.Sprintf("unsupported credential type %q", c.Type)}
	}
}

func wrapPing(kind connector.Kind, err error) PingResult {
	if err == nil {
		return PingResult{Kind: kind, OK: true}
	}
	return PingResult{Kind: kind, OK: false, Message: err.Error()}
}

// gitHostOnly is true when the remote is a bare hostname/base URL without a repo path
// (typical shared-pool git auth entry).
func gitHostOnly(remote string) bool {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return true
	}
	if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(remote, ":", 2)
		return len(parts) < 2 || strings.TrimSpace(parts[1]) == ""
	}
	if !strings.Contains(remote, "://") {
		return !strings.Contains(strings.Trim(remote, "/"), "/")
	}
	u, err := url.Parse(remote)
	if err != nil {
		return false
	}
	return strings.Trim(u.Path, "/") == ""
}

func pingTCPHost(ctx context.Context, remote string, port int) error {
	host := strings.TrimSpace(remote)
	scheme := "https"
	if strings.Contains(host, "://") {
		u, err := url.Parse(host)
		if err != nil {
			return err
		}
		if u.Scheme != "" {
			scheme = u.Scheme
		}
		host = u.Host
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if port <= 0 {
			if pi, e := strconv.Atoi(p); e == nil {
				port = pi
			}
		}
	}
	if port <= 0 {
		if scheme == "http" {
			port = 80
		} else {
			port = 443
		}
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("%w: %v", connector.ErrUnavailable, err)
	}
	_ = conn.Close()
	return nil
}
