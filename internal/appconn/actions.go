package appconn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"depman/internal/cache"
	"depman/internal/connector/git"
	"depman/internal/connector/rancher"
	"depman/internal/connector/registry"
	"depman/internal/model"
)

// GitFetch clones/fetches into the shared content cache and returns the worktree path.
func GitFetch(ctx context.Context, store *cache.DiskStore, c model.Credential, secret, ref string) (workDir string, err error) {
	if store == nil {
		return "", fmt.Errorf("cache not configured")
	}
	cfg := GitConfig(c, secret)
	key := cache.GitKey(cfg.Endpoint.BaseURL)
	// Do not EnsureDir before clone: PlainClone needs an absent/empty path without
	// conflicting leftovers; connector prepares the directory.
	dir := store.DirPath(key)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if err := git.New(cfg).EnsureWorktree(ctx, dir, ref); err != nil {
		return "", err
	}
	if _, err := store.EnsureDir(key); err != nil {
		// Worktree is usable; meta refresh failure should not hide success.
		return dir, nil
	}
	return dir, nil
}

// RegistryPull pulls an image into the content cache (docker-save tar).
func RegistryPull(ctx context.Context, store *cache.DiskStore, c model.Credential, secret string, ref registry.Ref) (localPath, cacheKey, digest string, err error) {
	if store == nil {
		return "", "", "", fmt.Errorf("cache not configured")
	}
	cli := registry.New(RegistryConfig(c, secret))
	digest, _ = cli.ResolveDigest(ctx, ref)

	host := registryHost(cli.Cfg.Endpoint.BaseURL)
	keyDigest := digest
	if keyDigest == "" {
		keyDigest = "tag/" + ref.String()
	}
	key := cache.OCIKey(host, ref.Repository, keyDigest)

	if ent, ok, _ := store.Get(ctx, key); ok {
		if _, statErr := os.Stat(ent.Path); statErr == nil {
			return ent.Path, key, digest, nil
		}
	}

	tmp := filepath.Join(store.Root(), "objects", fmt.Sprintf("%s.%d.pulling", cache.ObjectID(key), time.Now().UnixNano()))
	_ = os.MkdirAll(filepath.Dir(tmp), 0o755)
	if err := cli.Pull(ctx, ref, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", "", "", err
	}
	f, err := os.Open(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", "", "", err
	}
	ent, putErr := store.Put(ctx, key, f, cache.Meta{
		ContentType: "application/x-tar",
		Labels:      map[string]string{"kind": "oci-tar"},
	})
	_ = f.Close()
	_ = os.Remove(tmp)
	if putErr != nil {
		return "", "", "", putErr
	}
	if digest == "" {
		digest, _ = cli.ResolveDigest(ctx, ref)
	}
	return ent.Path, key, digest, nil
}

func registryHost(base string) string {
	h := strings.TrimSpace(base)
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	if i := strings.IndexByte(h, '/'); i >= 0 {
		h = h[:i]
	}
	return strings.ToLower(h)
}

// RegistryPush pushes a local tar to the registry.
func RegistryPush(ctx context.Context, c model.Credential, secret string, ref registry.Ref, localPath string) error {
	return registry.New(RegistryConfig(c, secret)).Push(ctx, ref, localPath)
}

// PathInCache reports whether path is inside the cache root (after Clean).
func PathInCache(store *cache.DiskStore, path string) bool {
	if store == nil || strings.TrimSpace(path) == "" {
		return false
	}
	root, err := filepath.Abs(store.Root())
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// RancherClient builds a rancher client from credential.
func RancherClient(c model.Credential, secret string) (*rancher.Client, error) {
	cfg, err := RancherConfig(c, secret)
	if err != nil {
		return nil, err
	}
	return rancher.New(cfg), nil
}
