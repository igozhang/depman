// Package registry is a portable OCI registry client (Harbor, Docker Hub, etc.).
package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"depman/internal/connector"
)

// Config targets an OCI-compatible registry.
type Config struct {
	Endpoint connector.Endpoint
	Auth     connector.Auth
}

// Ref is an image reference (repository + tag or digest).
type Ref struct {
	Repository string // e.g. library/nginx
	Tag        string // mutually exclusive with Digest when pushing by tag
	Digest     string // sha256:…
}

func (r Ref) String() string {
	repo := strings.Trim(strings.TrimSpace(r.Repository), "/")
	if d := strings.TrimSpace(r.Digest); d != "" {
		return repo + "@" + d
	}
	tag := strings.TrimSpace(r.Tag)
	if tag == "" {
		tag = "latest"
	}
	return repo + ":" + tag
}

// Client talks to an OCI registry.
type Client struct {
	Cfg Config
}

func New(cfg Config) *Client { return &Client{Cfg: cfg} }

func (c *Client) validate() error {
	if strings.TrimSpace(c.Cfg.Endpoint.BaseURL) == "" {
		return fmt.Errorf("%w: registry base URL empty", connector.ErrInvalidConfig)
	}
	return nil
}

func (c *Client) host() (string, error) {
	raw := strings.TrimSpace(c.Cfg.Endpoint.BaseURL)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%w: bad registry URL %q", connector.ErrInvalidConfig, c.Cfg.Endpoint.BaseURL)
	}
	return u.Host, nil
}

func (c *Client) nameOpts() []name.Option {
	if c.Cfg.Endpoint.Insecure {
		return []name.Option{name.Insecure}
	}
	return nil
}

func (c *Client) authenticator() authn.Authenticator {
	a := c.Cfg.Auth
	user := strings.TrimSpace(a.Username)
	secret := a.Secret
	if secret == "" && user == "" {
		return authn.Anonymous
	}
	switch a.Method {
	case connector.AuthBearer:
		if user == "" {
			return &authn.Basic{Username: "oauth2", Password: secret}
		}
		return &authn.Basic{Username: user, Password: secret}
	default:
		if user == "" {
			user = "oauth2"
		}
		return &authn.Basic{Username: user, Password: secret}
	}
}

func (c *Client) transport() http.RoundTripper {
	if !c.Cfg.Endpoint.Insecure {
		return remote.DefaultTransport
	}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // optional for internal registries
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func (c *Client) remoteOpts(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithContext(ctx),
		remote.WithAuth(c.authenticator()),
		remote.WithTransport(c.transport()),
	}
}

func (c *Client) parseRef(ref Ref) (name.Reference, error) {
	host, err := c.host()
	if err != nil {
		return nil, err
	}
	repo := strings.Trim(strings.TrimSpace(ref.Repository), "/")
	if repo == "" {
		return nil, fmt.Errorf("%w: repository empty", connector.ErrInvalidConfig)
	}
	var full string
	if d := strings.TrimSpace(ref.Digest); d != "" {
		full = host + "/" + repo + "@" + d
	} else {
		tag := strings.TrimSpace(ref.Tag)
		if tag == "" {
			tag = "latest"
		}
		full = host + "/" + repo + ":" + tag
	}
	return name.ParseReference(full, c.nameOpts()...)
}

// Ping authenticates against the registry using the OCI challenge flow
// (not a naive /v2/ Basic-only check, which false-fails on token registries).
func (c *Client) Ping(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	host, err := c.host()
	if err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(15 * time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reg, err := name.NewRegistry(host, c.nameOpts()...)
	if err != nil {
		return fmt.Errorf("%w: %v", connector.ErrInvalidConfig, err)
	}
	scopes := []string{reg.Scope(transport.PullScope)}
	_, err = transport.NewWithContext(ctx, reg, c.authenticator(), c.transport(), scopes)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") ||
			strings.Contains(msg, "403") || strings.Contains(msg, "denied") {
			return fmt.Errorf("%w: %v", connector.ErrAuth, err)
		}
		return fmt.Errorf("%w: %v", connector.ErrUnavailable, err)
	}
	return nil
}

// Pull downloads an image as a docker-save-compatible tarball at localPath.
func (c *Client) Pull(ctx context.Context, ref Ref, localPath string) error {
	if err := c.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(localPath) == "" {
		return fmt.Errorf("%w: localPath empty", connector.ErrInvalidConfig)
	}
	named, err := c.parseRef(ref)
	if err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(15 * time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	img, err := remote.Image(named, c.remoteOpts(ctx)...)
	if err != nil {
		return fmt.Errorf("%w: pull: %v", connector.ErrUnavailable, err)
	}
	if err := tarball.WriteToFile(localPath, named, img); err != nil {
		return fmt.Errorf("write tarball: %w", err)
	}
	return nil
}

// ResolveDigest returns the image digest for ref.
func (c *Client) ResolveDigest(ctx context.Context, ref Ref) (string, error) {
	named, err := c.parseRef(ref)
	if err != nil {
		return "", err
	}
	desc, err := remote.Head(named, c.remoteOpts(ctx)...)
	if err != nil {
		return "", err
	}
	return desc.Digest.String(), nil
}

// Push uploads a docker-save tarball from localPath to ref.
func (c *Client) Push(ctx context.Context, ref Ref, localPath string) error {
	if err := c.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(localPath) == "" {
		return fmt.Errorf("%w: localPath empty", connector.ErrInvalidConfig)
	}
	named, err := c.parseRef(ref)
	if err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(15 * time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	img, err := tarball.ImageFromPath(localPath, nil)
	if err != nil {
		return fmt.Errorf("read tarball: %w", err)
	}
	// Prefer tag destination; digest-only refs are written as-is.
	if err := remote.Write(named, img, c.remoteOpts(ctx)...); err != nil {
		return fmt.Errorf("%w: push: %v", connector.ErrUnavailable, err)
	}
	return nil
}
