package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// OCIKey builds a content key for an OCI image reference by digest.
// host and repository should be normalized (no scheme); digest like "sha256:...".
func OCIKey(host, repository, digest string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	digest = strings.TrimSpace(digest)
	return "oci/" + host + "/" + repository + "@" + digest
}

// GitKey builds a content key for a git remote URL (normalized).
func GitKey(remoteURL string) string {
	n := normalizeGitURL(remoteURL)
	sum := sha256.Sum256([]byte(n))
	return "git/" + hex.EncodeToString(sum[:16])
}

func normalizeGitURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// git@host:path → ssh://git@host/path
	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		if i := strings.IndexByte(rest, ':'); i > 0 {
			raw = "ssh://git@" + rest[:i] + "/" + rest[i+1:]
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(strings.ToLower(raw), "/")
	}
	u.User = nil
	u.Fragment = ""
	u.RawQuery = ""
	u.Path = strings.TrimRight(u.Path, "/")
	u.Host = strings.ToLower(u.Host)
	u.Scheme = strings.ToLower(u.Scheme)
	return u.String()
}

// ObjectID returns a filesystem-safe id for an arbitrary cache key.
func ObjectID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
