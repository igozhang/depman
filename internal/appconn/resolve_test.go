package appconn

import (
	"path/filepath"
	"testing"

	"depman/internal/cache"
	"depman/internal/connector"
	"depman/internal/model"
)

func cacheOpen(dir string) (*cache.DiskStore, error) {
	return cache.OpenDisk(filepath.Join(dir, "c"))
}

func TestBaseURL(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"harbor.example", 0, "https://harbor.example"},
		{"harbor.example", 443, "https://harbor.example"},
		{"harbor.example", 8443, "https://harbor.example:8443"},
		{"harbor.example:8443", 8443, "https://harbor.example:8443"},
		{"harbor.example:8443", 0, "https://harbor.example:8443"},
		{"https://harbor.example:8443/", 0, "https://harbor.example:8443"},
		{"https://harbor.example", 8443, "https://harbor.example"},
		{"10.0.0.1", 80, "http://10.0.0.1"},
	}
	for _, tc := range cases {
		got := baseURL(tc.host, tc.port, "https")
		if got != tc.want {
			t.Fatalf("baseURL(%q,%d)=%q want %q", tc.host, tc.port, got, tc.want)
		}
	}
}

func TestRegistryBearerWhenNoUser(t *testing.T) {
	cfg := RegistryConfig(model.Credential{Host: "harbor.example", Username: ""}, "token-only")
	if cfg.Auth.Method != connector.AuthBearer {
		t.Fatalf("method=%s", cfg.Auth.Method)
	}
	cfg2 := RegistryConfig(model.Credential{Host: "harbor.example", Username: "robot"}, "pass")
	if cfg2.Auth.Method != connector.AuthBasic {
		t.Fatalf("method=%s", cfg2.Auth.Method)
	}
}

func TestGitConfigRepoJoin(t *testing.T) {
	cfg := GitConfig(model.Credential{
		Host:  "gitee.com",
		Extra: `{"repo":"org/app.git","branch":"main"}`,
	}, "pat")
	if cfg.Endpoint.BaseURL != "https://gitee.com/org/app.git" {
		t.Fatalf("remote=%s", cfg.Endpoint.BaseURL)
	}
	if cfg.Branch != "main" {
		t.Fatalf("branch=%s", cfg.Branch)
	}
	if !cfg.Endpoint.Insecure {
		t.Fatal("expected default Insecure for internal HTTPS")
	}
}

func TestGitConfigHTTPPort(t *testing.T) {
	cfg := GitConfig(model.Credential{Host: "gitlab.example.com", Port: 80}, "pat")
	if cfg.Endpoint.BaseURL != "http://gitlab.example.com" {
		t.Fatalf("remote=%s", cfg.Endpoint.BaseURL)
	}
}

func TestGitConfigExplicitHTTPS(t *testing.T) {
	cfg := GitConfig(model.Credential{Host: "https://gitlab.example.com"}, "pat")
	if cfg.Endpoint.BaseURL != "https://gitlab.example.com" {
		t.Fatalf("remote=%s", cfg.Endpoint.BaseURL)
	}
	secure := false
	cfg2 := GitConfig(model.Credential{
		Host:  "https://gitlab.example.com",
		Extra: `{"insecure":false}`,
	}, "pat")
	if cfg2.Endpoint.Insecure != secure {
		t.Fatalf("Insecure=%v want false", cfg2.Endpoint.Insecure)
	}
}

func TestGitConfigSchemeRepoJoin(t *testing.T) {
	cfg := GitConfig(model.Credential{
		Host:  "http://gitlab.example.com",
		Extra: `{"repo":"group/app.git"}`,
	}, "pat")
	if cfg.Endpoint.BaseURL != "http://gitlab.example.com/group/app.git" {
		t.Fatalf("remote=%s", cfg.Endpoint.BaseURL)
	}
}

func TestRancherRejectsOtherProvider(t *testing.T) {
	_, err := RancherConfig(model.Credential{
		Host:  "https://example",
		Extra: `{"provider":"openshift"}`,
	}, "tok")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitConfigHTTPSKeepsBasicDespitePEM(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nxxx\n-----END OPENSSH PRIVATE KEY-----"
	cfg := GitConfig(model.Credential{Host: "https://gitee.com/org/repo.git"}, pem)
	if cfg.Auth.Method != connector.AuthBasic {
		t.Fatalf("https URL should use basic, got %s", cfg.Auth.Method)
	}
}

func TestGitConfigSSHURLUsesKey(t *testing.T) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\nxxx\n-----END OPENSSH PRIVATE KEY-----"
	cfg := GitConfig(model.Credential{Host: "git@gitee.com:org/repo.git"}, pem)
	if cfg.Auth.Method != connector.AuthSSHKey {
		t.Fatalf("ssh URL should use ssh_key, got %s", cfg.Auth.Method)
	}
}

func TestPathInCache(t *testing.T) {
	dir := t.TempDir()
	s, err := cacheOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(s.Root(), "objects", "x")
	if !PathInCache(s, inside) {
		t.Fatal("expected inside")
	}
	if PathInCache(s, filepath.Join(dir, "..", "outside")) {
		t.Fatal("expected reject outside")
	}
}
