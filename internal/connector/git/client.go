// Package git provides read-only remote access (clone / fetch).
package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"golang.org/x/crypto/ssh"

	"depman/internal/connector"
)

// Config targets a git remote.
type Config struct {
	Endpoint connector.Endpoint // BaseURL = remote URL
	Auth     connector.Auth
	Branch   string // optional default branch/ref
}

// Client is a read-only git connector.
type Client struct {
	Cfg Config
}

func New(cfg Config) *Client { return &Client{Cfg: cfg} }

func (c *Client) validate() error {
	if strings.TrimSpace(c.Cfg.Endpoint.BaseURL) == "" {
		return fmt.Errorf("%w: git remote URL empty", connector.ErrInvalidConfig)
	}
	return nil
}

func (c *Client) authMethod() (transport.AuthMethod, error) {
	a := c.Cfg.Auth
	switch a.Method {
	case connector.AuthNone, "":
		if strings.TrimSpace(a.Secret) == "" {
			return nil, nil
		}
		fallthrough
	case connector.AuthBasic, connector.AuthBearer:
		user := strings.TrimSpace(a.Username)
		if user == "" {
			user = "git"
		}
		return &githttp.BasicAuth{Username: user, Password: a.Secret}, nil
	case connector.AuthSSHKey:
		user := strings.TrimSpace(a.Username)
		if user == "" {
			user = "git"
		}
		pubKeys, err := gitssh.NewPublicKeys(user, []byte(a.Secret), "")
		if err != nil {
			return nil, fmt.Errorf("%w: parse ssh key: %v", connector.ErrInvalidConfig, err)
		}
		pubKeys.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // ops tool
		return pubKeys, nil
	default:
		return nil, fmt.Errorf("%w: unsupported git auth %q", connector.ErrInvalidConfig, a.Method)
	}
}

// Ping runs ls-remote against the configured remote.
func (c *Client) Ping(ctx context.Context) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	auth, err := c.authMethod()
	if err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(30 * time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rem := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{c.Cfg.Endpoint.BaseURL},
	})
	_, err = rem.ListContext(ctx, &gogit.ListOptions{Auth: auth})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "authentication") ||
			strings.Contains(err.Error(), "401") ||
			strings.Contains(err.Error(), "403") {
			return fmt.Errorf("%w: %v", connector.ErrAuth, err)
		}
		return fmt.Errorf("%w: %v", connector.ErrUnavailable, err)
	}
	return nil
}

// EnsureWorktree clones or fetches into workDir (read-only use of remote).
func (c *Client) EnsureWorktree(ctx context.Context, workDir, ref string) error {
	if err := c.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("%w: workDir empty", connector.ErrInvalidConfig)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if ref == "" {
		ref = c.Cfg.Branch
	}
	auth, err := c.authMethod()
	if err != nil {
		return err
	}
	timeout := c.Cfg.Endpoint.TimeoutOr(10 * time.Minute)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	repo, err := gogit.PlainOpen(workDir)
	if err != nil {
		if err := prepareCloneDir(workDir); err != nil {
			return err
		}
		// Fresh clone into workDir.
		opts := &gogit.CloneOptions{
			URL:           c.Cfg.Endpoint.BaseURL,
			Auth:          auth,
			SingleBranch:  ref != "",
			ReferenceName: refName(ref),
			Depth:         1,
		}
		_, err = gogit.PlainCloneContext(ctx, workDir, false, opts)
		if err != nil {
			// Retry without depth/single-branch for servers that reject shallow.
			_ = os.RemoveAll(workDir)
			if err := prepareCloneDir(workDir); err != nil {
				return err
			}
			opts.Depth = 0
			opts.SingleBranch = false
			opts.ReferenceName = ""
			repo, err = gogit.PlainCloneContext(ctx, workDir, false, opts)
			if err != nil {
				return fmt.Errorf("%w: clone: %v", connector.ErrUnavailable, err)
			}
			return checkout(repo, ref)
		}
		return nil
	}

	// Existing repo: fetch + checkout.
	err = repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      true,
		Tags:       gogit.AllTags,
	})
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("%w: fetch: %v", connector.ErrUnavailable, err)
	}
	return checkout(repo, ref)
}

// prepareCloneDir ensures workDir exists and is empty enough for PlainClone.
// Leftover non-git content from a failed run is removed.
func prepareCloneDir(workDir string) error {
	st, err := os.Stat(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(workDir, 0o755)
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%w: workDir is not a directory", connector.ErrInvalidConfig)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".git")); err == nil {
		return nil
	}
	ents, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}
	if len(ents) == 0 {
		return nil
	}
	if err := os.RemoveAll(workDir); err != nil {
		return err
	}
	return os.MkdirAll(workDir, 0o755)
}

func refName(ref string) plumbing.ReferenceName {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "refs/") {
		return plumbing.ReferenceName(ref)
	}
	if strings.HasPrefix(ref, "tags/") {
		return plumbing.ReferenceName("refs/" + ref)
	}
	return plumbing.NewBranchReferenceName(ref)
}

func checkout(repo *gogit.Repository, ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	// Try local branch, then remote, then tag, then raw hash.
	candidates := []plumbing.ReferenceName{
		refName(ref),
		plumbing.NewRemoteReferenceName("origin", ref),
		plumbing.NewTagReferenceName(ref),
	}
	var last error
	for _, name := range candidates {
		if name == "" {
			continue
		}
		err := wt.Checkout(&gogit.CheckoutOptions{Branch: name, Force: true})
		if err == nil {
			return nil
		}
		last = err
	}
	// Hash / detached
	hash := plumbing.NewHash(ref)
	if !hash.IsZero() {
		if err := wt.Checkout(&gogit.CheckoutOptions{Hash: hash, Force: true}); err == nil {
			return nil
		} else {
			last = err
		}
	}
	if last != nil {
		return fmt.Errorf("checkout %q: %w", ref, last)
	}
	return fmt.Errorf("checkout %q failed", ref)
}
