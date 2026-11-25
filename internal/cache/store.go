// Package cache provides a content-keyed file cache with TTL GC.
// It has no knowledge of environments, projects, or credentials.
package cache

import (
	"context"
	"errors"
	"io"
	"time"
)

const (
	// LargeFileBytes: objects at or above this size use a shorter TTL.
	LargeFileBytes int64 = 1 << 30 // 1 GiB
	LargeFileTTL         = 7 * 24 * time.Hour
	DefaultTTL           = 30 * 24 * time.Hour
)

var (
	ErrNotFound = errors.New("cache: not found")
	ErrBusy     = errors.New("cache: entry busy")
)

// Meta describes a cached object (beyond the blob itself).
type Meta struct {
	ContentType string            // optional hint, e.g. application/vnd.oci.image
	Labels      map[string]string // optional free-form labels
}

// Entry is a cache lookup result.
type Entry struct {
	Key        string
	Path       string
	Size       int64
	CreatedAt  time.Time
	LastAccess time.Time
	Busy       bool
	Meta       Meta
}

// Store is a pluggable content cache. Keys are opaque strings
// (e.g. "oci/harbor.example/lib/app@sha256:abcd", "git/<hash>").
// Cross-tenant reuse is allowed at this layer; callers that need
// isolation must encode it into the key themselves.
type Store interface {
	Root() string
	Path(key string) string
	Get(ctx context.Context, key string) (Entry, bool, error)
	Put(ctx context.Context, key string, r io.Reader, meta Meta) (Entry, error)
	Touch(ctx context.Context, key string) error
	SetBusy(ctx context.Context, key string, busy bool) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]Entry, error)
	GC(ctx context.Context, now time.Time) (removed int, freed int64, err error)
}

// TTLForSize returns the retention window for an object of the given size.
func TTLForSize(size int64) time.Duration {
	if size >= LargeFileBytes {
		return LargeFileTTL
	}
	return DefaultTTL
}

// Expired reports whether lastAccess+TTL(size) is before now.
func Expired(size int64, lastAccess, now time.Time) bool {
	if lastAccess.IsZero() {
		return true
	}
	return now.After(lastAccess.Add(TTLForSize(size)))
}
