package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DirPath returns the on-disk directory for a content key (e.g. git worktree).
// Layout: <root>/dirs/<objectID>/
func (d *DiskStore) DirPath(key string) string {
	return filepath.Join(d.root, "dirs", ObjectID(key))
}

// EnsureDir creates the directory for key and refreshes dir meta (for TTL GC).
func (d *DiskStore) EnsureDir(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("cache: empty key")
	}
	if err := os.MkdirAll(filepath.Join(d.root, "dirs"), 0o755); err != nil {
		return "", err
	}
	p := d.DirPath(key)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	if err := d.touchDirMeta(key, p); err != nil {
		return p, err
	}
	return p, nil
}

// TouchDir refreshes last_access for a directory cache entry.
func (d *DiskStore) TouchDir(key string) error {
	p := d.DirPath(key)
	st, err := os.Stat(p)
	if err != nil {
		return mapExist(err)
	}
	if !st.IsDir() {
		return fmt.Errorf("cache: not a directory")
	}
	return d.touchDirMeta(key, p)
}

func (d *DiskStore) touchDirMeta(key, path string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	size := dirSize(path)
	now := time.Now().UTC()
	m := diskMeta{
		Key:         key,
		Size:        size,
		CreatedAt:   now,
		LastAccess:  now,
		ContentType: "inode/directory",
		Labels:      map[string]string{"kind": "dir"},
	}
	if old, err := d.readMeta(key); err == nil {
		m.CreatedAt = old.CreatedAt
		m.Busy = old.Busy
	}
	return d.writeMeta(m)
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
