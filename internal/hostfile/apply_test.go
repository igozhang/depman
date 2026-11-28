package hostfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyManagedBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1\tlocalhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Apply(path, []Entry{{IP: "10.0.0.1", Hostname: "gitlab.example.com"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "127.0.0.1\tlocalhost") {
		t.Fatalf("lost original: %s", s)
	}
	if !strings.Contains(s, beginMarker) || !strings.Contains(s, "10.0.0.1\tgitlab.example.com") {
		t.Fatalf("missing managed entry: %s", s)
	}

	if err := Apply(path, []Entry{{IP: "10.0.0.2", Hostname: "harbor.example.com"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	s = string(raw)
	if strings.Contains(s, "gitlab.example.com") {
		t.Fatalf("old entry not replaced: %s", s)
	}
	if !strings.Contains(s, "10.0.0.2\tharbor.example.com") {
		t.Fatalf("new entry missing: %s", s)
	}

	if err := Apply(path, nil); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	s = string(raw)
	if strings.Contains(s, beginMarker) {
		t.Fatalf("managed block should be removed: %s", s)
	}
	if !strings.Contains(s, "127.0.0.1\tlocalhost") {
		t.Fatalf("lost original after clear: %s", s)
	}
}
