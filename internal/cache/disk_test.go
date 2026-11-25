package cache

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDiskPutGetGC(t *testing.T) {
	root := t.TempDir()
	s, err := OpenDisk(filepath.Join(root, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := OCIKey("harbor.example", "lib/app", "sha256:abc")
	ent, err := s.Put(ctx, key, bytes.NewReader([]byte("hello")), Meta{ContentType: "application/octet-stream"})
	if err != nil {
		t.Fatal(err)
	}
	if ent.Size != 5 {
		t.Fatalf("size %d", ent.Size)
	}
	got, ok, err := s.Get(ctx, key)
	if err != nil || !ok || got.Key != key {
		t.Fatalf("get: ok=%v err=%v key=%q", ok, err, got.Key)
	}
	// Get must refresh last_access (TTL clock).
	s.mu.Lock()
	mTouch, _ := s.readMeta(key)
	s.mu.Unlock()
	if mTouch.LastAccess.Before(ent.LastAccess) {
		t.Fatalf("Get did not refresh last_access")
	}

	// Force expire small file by backdating last access beyond DefaultTTL.
	s.mu.Lock()
	m, _ := s.readMeta(key)
	m.LastAccess = time.Now().UTC().Add(-31 * 24 * time.Hour)
	_ = s.writeMeta(m)
	s.mu.Unlock()

	n, freed, err := s.GC(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || freed != 5 {
		t.Fatalf("gc n=%d freed=%d", n, freed)
	}
	_, ok, err = s.Get(ctx, key)
	if err != nil || ok {
		t.Fatalf("expected removed, ok=%v err=%v", ok, err)
	}
}
