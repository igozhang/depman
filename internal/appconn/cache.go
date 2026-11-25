package appconn

import (
	"path/filepath"

	"depman/internal/cache"
)

// DefaultCacheDir returns <dataDir>/cache.
func DefaultCacheDir(dataDir string) string {
	return filepath.Join(dataDir, "cache")
}

// OpenCache opens the shared content cache (cross-env reuse by content key).
func OpenCache(cacheDir string) (*cache.DiskStore, error) {
	return cache.OpenDisk(cacheDir)
}
