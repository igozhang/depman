package store

import (
	"path/filepath"
	"testing"

	"depman/internal/model"
)

func TestSharedCredentialAndAuthRef(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	env, err := st.CreateEnvironment(model.EnvironmentInput{Name: "dev1", Kind: "dev"})
	if err != nil {
		t.Fatal(err)
	}

	shared, err := st.CreateCredential(model.CredentialInput{
		Name:     "gitlab",
		Type:     "git",
		Host:     "gitlab.example.com",
		Username: "bot",
		Secret:   "token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if shared.EnvID != "" {
		t.Fatalf("shared envId=%q", shared.EnvID)
	}

	target, err := st.CreateCredential(model.CredentialInput{
		EnvID:      env.ID,
		Name:       "pyscripts",
		Type:       "git",
		Host:       "https://gitlab.example.com/g/repo.git",
		AuthCredID: shared.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, secret, err := st.ResolveAuth(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "https://gitlab.example.com/g/repo.git" {
		t.Fatalf("host=%s", got.Host)
	}
	if got.Username != "bot" || secret != "token" {
		t.Fatalf("auth user=%s secret=%s", got.Username, secret)
	}

	if err := st.DeleteCredential(shared.ID); err == nil {
		t.Fatal("expected refuse delete while referenced")
	}
	if err := st.DeleteCredential(target.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteCredential(shared.ID); err != nil {
		t.Fatal(err)
	}
}

func TestSharedCannotNestAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, err := st.CreateCredential(model.CredentialInput{Name: "a", Type: "git", Host: "g.example", Username: "u", Secret: "p"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateCredential(model.CredentialInput{Name: "b", Type: "git", Host: "g.example", AuthCredID: a.ID})
	if err == nil {
		t.Fatal("shared must not nest auth")
	}
}

func TestProjectEnvAcceptsSharedSSH(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.CreateProject(model.ProjectInput{Name: "p"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := st.CreateEnvironment(model.EnvironmentInput{Name: "e", Kind: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	ssh, err := st.CreateCredential(model.CredentialInput{Name: "jump", Type: "ssh", Host: "10.1.1.1", Port: 22, Username: "root", Secret: "key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.UpsertProjectEnv(model.ProjectEnvInput{
		ProjectID: p.ID,
		EnvID:     env.ID,
		Name:      "task",
		SSHCredID: ssh.ID,
		Script:    "echo ok",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListCredentialsSharedFilter(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	env, err := st.CreateEnvironment(model.EnvironmentInput{Name: "e", Kind: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateCredential(model.CredentialInput{Name: "s", Type: "git", Host: "h", Username: "u", Secret: "p"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateCredential(model.CredentialInput{EnvID: env.ID, Name: "e1", Type: "ssh", Host: "10.0.0.1", Port: 22, Secret: "k"})
	if err != nil {
		t.Fatal(err)
	}
	shared, err := st.ListCredentials("shared", false)
	if err != nil || len(shared) != 1 {
		t.Fatalf("shared=%d err=%v", len(shared), err)
	}
	envCreds, err := st.ListCredentials(env.ID, false)
	if err != nil || len(envCreds) != 1 {
		t.Fatalf("env=%d err=%v", len(envCreds), err)
	}
}
