package appconn

import "testing"

func TestResolveHTTPDialTarget(t *testing.T) {
	cases := []struct {
		remote string
		port   int
		host   string
		want   int
	}{
		{"https://gitlab.example.com", 0, "gitlab.example.com", 443},
		{"http://gitlab.example.com", 0, "gitlab.example.com", 80},
		// Scheme in URL wins over a stale credential port (matches baseURL).
		{"http://gitlab.example.com", 443, "gitlab.example.com", 80},
		{"https://gitlab.example.com", 80, "gitlab.example.com", 443},
		{"https://gitlab.example.com:8443", 0, "gitlab.example.com", 8443},
		{"http://gitlab.example.com:8080/", 443, "gitlab.example.com", 8080},
		// Bare host without scheme still honors credential port.
		{"gitlab.example.com", 80, "gitlab.example.com", 80},
		{"gitlab.example.com", 0, "gitlab.example.com", 443},
	}
	for _, tc := range cases {
		host, port, err := resolveHTTPDialTarget(tc.remote, tc.port)
		if err != nil {
			t.Fatalf("remote=%q port=%d: %v", tc.remote, tc.port, err)
		}
		if host != tc.host || port != tc.want {
			t.Fatalf("remote=%q credPort=%d → %s:%d want %s:%d",
				tc.remote, tc.port, host, port, tc.host, tc.want)
		}
	}
}

func TestResolveSSHDialTarget(t *testing.T) {
	cases := []struct {
		remote string
		port   int
		host   string
		want   int
	}{
		{"git@gitlab.example.com", 0, "gitlab.example.com", 22},
		{"git@gitlab.example.com:group/repo.git", 0, "gitlab.example.com", 22},
		{"ssh://git@gitlab.example.com/group/repo.git", 0, "gitlab.example.com", 22},
		{"ssh://git@gitlab.example.com:2222/group/repo.git", 0, "gitlab.example.com", 2222},
		{"gitlab.example.com", 0, "gitlab.example.com", 22},
		{"https://gitlab.example.com", 0, "gitlab.example.com", 22},
	}
	for _, tc := range cases {
		host, port, err := resolveSSHDialTarget(tc.remote, tc.port)
		if err != nil {
			t.Fatalf("remote=%q: %v", tc.remote, err)
		}
		if host != tc.host || port != tc.want {
			t.Fatalf("remote=%q → %s:%d want %s:%d", tc.remote, host, port, tc.host, tc.want)
		}
	}
}

func TestGitHostOnly(t *testing.T) {
	if !gitHostOnly("https://gitlab.example.com") {
		t.Fatal("expected host-only")
	}
	if gitHostOnly("https://gitlab.example.com/group/repo.git") {
		t.Fatal("expected repo path")
	}
	if !gitHostOnly("git@gitlab.example.com") {
		t.Fatal("expected host-only scp")
	}
	if gitHostOnly("git@gitlab.example.com:group/repo.git") {
		t.Fatal("expected scp with path")
	}
}
