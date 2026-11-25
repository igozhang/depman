package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type diskMeta struct {
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"createdAt"`
	LastAccess  time.Time         `json:"lastAccess"`
	Busy        bool              `json:"busy"`
	ContentType string            `json:"contentType,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// DiskStore stores blobs under Root/objects and JSON sidecars under Root/meta.
type DiskStore struct {
	root string
	mu   sync.Mutex
}

// OpenDisk creates (if needed) and returns a disk-backed Store.
func OpenDisk(root string) (*DiskStore, error) {
	root = filepath.Clean(root)
	for _, sub := range []string{"objects", "meta", "dirs"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &DiskStore{root: root}, nil
}

func (d *DiskStore) Root() string { return d.root }

func (d *DiskStore) Path(key string) string {
	return filepath.Join(d.root, "objects", ObjectID(key))
}

func (d *DiskStore) metaPath(key string) string {
	return filepath.Join(d.root, "meta", ObjectID(key)+".json")
}

func (d *DiskStore) Get(_ context.Context, key string) (Entry, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, err := d.readMeta(key)
	if err != nil {
		if os.IsNotExist(err) {
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}
	p := d.Path(key)
	if m.Labels != nil && m.Labels["kind"] == "dir" {
		p = filepath.Join(d.root, "dirs", ObjectID(key))
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			_ = os.Remove(d.metaPath(key))
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}
	// Access refreshes TTL (last_access).
	m.LastAccess = time.Now().UTC()
	if err := d.writeMeta(m); err != nil {
		return d.toEntry(m, p), true, nil
	}
	return d.toEntry(m, p), true, nil
}

func (d *DiskStore) Put(_ context.Context, key string, r io.Reader, meta Meta) (Entry, error) {
	if key == "" {
		return Entry{}, fmt.Errorf("cache: empty key")
	}
	obj := d.Path(key)
	// Unique tmp so large copies do not need the global mutex, and concurrent Puts do not clobber one tmp.
	tmp := fmt.Sprintf("%s.%d.tmp", obj, time.Now().UnixNano())
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return Entry{}, err
	}
	n, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return Entry{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return Entry{}, closeErr
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := os.Rename(tmp, obj); err != nil {
		_ = os.Remove(tmp)
		return Entry{}, err
	}

	now := time.Now().UTC()
	dm := diskMeta{
		Key:         key,
		Size:        n,
		CreatedAt:   now,
		LastAccess:  now,
		ContentType: meta.ContentType,
		Labels:      meta.Labels,
	}
	if err := d.writeMeta(dm); err != nil {
		_ = os.Remove(obj)
		return Entry{}, err
	}
	return d.toEntry(dm, obj), nil
}

func (d *DiskStore) Touch(_ context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, err := d.readMeta(key)
	if err != nil {
		return mapExist(err)
	}
	m.LastAccess = time.Now().UTC()
	return d.writeMeta(m)
}

func (d *DiskStore) SetBusy(_ context.Context, key string, busy bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	m, err := d.readMeta(key)
	if err != nil {
		return mapExist(err)
	}
	m.Busy = busy
	return d.writeMeta(m)
}

func (d *DiskStore) Delete(_ context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deleteLocked(key)
}

func (d *DiskStore) deleteLocked(key string) error {
	m, err := d.readMeta(key)
	if err != nil {
		return mapExist(err)
	}
	if m.Busy {
		return ErrBusy
	}
	_ = os.Remove(d.Path(key))
	if m.Labels != nil && m.Labels["kind"] == "dir" {
		_ = os.RemoveAll(filepath.Join(d.root, "dirs", ObjectID(key)))
	}
	_ = os.Remove(d.metaPath(key))
	return nil
}

func (d *DiskStore) List(_ context.Context) ([]Entry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	dir := filepath.Join(d.root, "meta")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m diskMeta
		if json.Unmarshal(b, &m) != nil || m.Key == "" {
			continue
		}
		out = append(out, d.toEntry(m, d.Path(m.Key)))
	}
	return out, nil
}

func (d *DiskStore) GC(ctx context.Context, now time.Time) (int, int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ents, err := d.List(ctx)
	if err != nil {
		return 0, 0, err
	}
	var removed int
	var freed int64
	for _, e := range ents {
		if err := ctx.Err(); err != nil {
			return removed, freed, err
		}
		if e.Busy {
			continue
		}
		if !Expired(e.Size, e.LastAccess, now) {
			continue
		}
		if err := d.Delete(ctx, e.Key); err != nil {
			// Race with another GC/user delete, or busy flipped — keep going.
			if err == ErrBusy || err == ErrNotFound {
				continue
			}
			return removed, freed, err
		}
		removed++
		freed += e.Size
	}
	return removed, freed, nil
}

func (d *DiskStore) readMeta(key string) (diskMeta, error) {
	b, err := os.ReadFile(d.metaPath(key))
	if err != nil {
		return diskMeta{}, err
	}
	var m diskMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return diskMeta{}, err
	}
	return m, nil
}

func (d *DiskStore) writeMeta(m diskMeta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := d.metaPath(m.Key)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (d *DiskStore) toEntry(m diskMeta, path string) Entry {
	return Entry{
		Key:        m.Key,
		Path:       path,
		Size:       m.Size,
		CreatedAt:  m.CreatedAt,
		LastAccess: m.LastAccess,
		Busy:       m.Busy,
		Meta: Meta{
			ContentType: m.ContentType,
			Labels:      m.Labels,
		},
	}
}

func mapExist(err error) error {
	if os.IsNotExist(err) {
		return ErrNotFound
	}
	return err
}
