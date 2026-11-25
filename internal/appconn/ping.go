package appconn

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"depman/internal/connector"
	"depman/internal/connector/git"
	"depman/internal/connector/rancher"
	"depman/internal/connector/registry"
	connssh "depman/internal/connector/ssh"
	"depman/internal/model"
)

// PingResult is a normalized connectivity probe response.
//
// OK means no hard failure. When Stub is true, the remote was NOT contacted —
// only local config validation passed; callers must treat Stub as “not a real probe”.
type PingResult struct {
	Kind    connector.Kind `json:"kind"`
	OK      bool           `json:"ok"`
	Stub    bool           `json:"stub,omitempty"`
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
		err := git.New(GitConfig(c, secret)).Ping(ctx)
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
	if errors.Is(err, connector.ErrNotImplemented) {
		return PingResult{
			Kind:    kind,
			OK:      true,
			Stub:    true,
			Message: "skeleton: config ok, remote probe pending",
		}
	}
	return PingResult{Kind: kind, OK: false, Message: err.Error()}
}
