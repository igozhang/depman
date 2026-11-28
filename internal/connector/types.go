// Package connector defines portable integration clients (registry, git, ssh, cluster).
// It does not load credentials from a database; callers inject Auth/Endpoint.
package connector

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidConfig = errors.New("connector: invalid config")
	ErrAuth          = errors.New("connector: auth failed")
	ErrUnavailable   = errors.New("connector: unavailable")
)

// AuthMethod describes how Secret is interpreted.
type AuthMethod string

const (
	AuthNone   AuthMethod = "none"
	AuthBasic  AuthMethod = "basic"  // Username + Secret as password
	AuthBearer AuthMethod = "bearer" // Secret as bearer/token (Username optional)
	AuthSSHKey AuthMethod = "ssh_key" // Secret as PEM private key
)

// Endpoint is a remote service location.
type Endpoint struct {
	BaseURL  string
	Timeout  time.Duration
	Insecure bool // skip TLS verify (common for internal Harbor/Rancher)
}

func (e Endpoint) TimeoutOr(def time.Duration) time.Duration {
	if e.Timeout > 0 {
		return e.Timeout
	}
	return def
}

// Auth holds credentials for a connector call.
type Auth struct {
	Username string
	Secret   string
	Method   AuthMethod
}

// Pinger is the minimal contract every connector should satisfy.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Kind identifies a connector family (stable string for APIs/logs).
type Kind string

const (
	KindRegistry Kind = "registry" // OCI registries (Harbor, Docker Hub, …)
	KindGit      Kind = "git"
	KindSSH      Kind = "ssh"
	KindCluster  Kind = "cluster" // Kubernetes-facing management APIs
)
