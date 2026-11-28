// Package hostfile syncs managed hostname→IP entries into the pod hosts file.
package hostfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "# BEGIN depman-host-resolves"
	endMarker   = "# END depman-host-resolves"
)

// Entry is one hosts-file mapping.
type Entry struct {
	IP       string
	Hostname string
}

// Path returns the hosts file path (DEPMAN_HOSTS_FILE or /etc/hosts).
func Path() string {
	if p := strings.TrimSpace(os.Getenv("DEPMAN_HOSTS_FILE")); p != "" {
		return p
	}
	return "/etc/hosts"
}

// Apply rewrites the managed block in path to match entries.
// Empty entries removes the managed block.
func Apply(path string, entries []Entry) error {
	if path == "" {
		path = Path()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read hosts: %w", err)
		}
		raw = nil
	}
	base := stripManagedBlock(string(raw))
	block := buildBlock(entries)
	out := strings.TrimRight(base, "\n")
	if block != "" {
		if out != "" {
			out += "\n\n"
		}
		out += block
	}
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".depman.tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		// Some mounts disallow rename over /etc/hosts; fall back to truncate-write.
		if err2 := os.WriteFile(path, []byte(out), 0o644); err2 != nil {
			return fmt.Errorf("write hosts: %v (rename: %v)", err2, err)
		}
	}
	return nil
}

func stripManagedBlock(s string) string {
	for {
		start := strings.Index(s, beginMarker)
		if start < 0 {
			return strings.TrimRight(s, "\n")
		}
		end := strings.Index(s[start:], endMarker)
		if end < 0 {
			return strings.TrimRight(s[:start], "\n")
		}
		end = start + end + len(endMarker)
		if end < len(s) && s[end] == '\n' {
			end++
		}
		s = s[:start] + s[end:]
	}
}

func buildBlock(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(beginMarker)
	b.WriteByte('\n')
	for _, e := range entries {
		ip := strings.TrimSpace(e.IP)
		host := strings.ToLower(strings.TrimSpace(e.Hostname))
		if ip == "" || host == "" {
			continue
		}
		b.WriteString(ip)
		b.WriteByte('\t')
		b.WriteString(host)
		b.WriteByte('\n')
	}
	b.WriteString(endMarker)
	b.WriteByte('\n')
	return b.String()
}
