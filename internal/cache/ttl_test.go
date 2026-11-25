package cache

import (
	"testing"
	"time"
)

func TestTTLForSize(t *testing.T) {
	if TTLForSize(LargeFileBytes) != LargeFileTTL {
		t.Fatalf("large ttl")
	}
	if TTLForSize(LargeFileBytes-1) != DefaultTTL {
		t.Fatalf("default ttl")
	}
}

func TestExpired(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	access := now.Add(-8 * 24 * time.Hour)
	if !Expired(LargeFileBytes, access, now) {
		t.Fatal("large file should expire after 7d")
	}
	if Expired(100, access, now) {
		t.Fatal("small file should still be valid at 8d < 30d")
	}
}

func TestGitKeyStable(t *testing.T) {
	a := GitKey("https://github.com/org/repo.git")
	b := GitKey("https://github.com/org/repo.git/")
	if a != b {
		t.Fatalf("normalize failed: %s vs %s", a, b)
	}
	if a != GitKey("https://user:pass@github.com/org/repo.git") {
		t.Fatal("userinfo should be stripped")
	}
}
