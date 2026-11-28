package store

import (
	"path/filepath"
	"testing"
)

// Old PVC DBs may still have credentials without auth_cred_id; Open must not crash.
func TestOpenUpgradesLegacyCredentialsSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy.db")

	st, err := Open(dbPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.db.Exec(`
DROP TABLE credentials;
CREATE TABLE credentials (
  id TEXT PRIMARY KEY,
  env_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  secret_enc TEXT NOT NULL DEFAULT '',
  extra TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
INSERT INTO credentials(id,env_id,name,type,host,port,username,secret_enc,extra,created_at,updated_at)
VALUES('c1','e1','n','ssh','h',22,'u','','',1,1);
`)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	st2, err := Open(dbPath, dir)
	if err != nil {
		t.Fatalf("reopen after legacy schema: %v", err)
	}
	defer st2.Close()

	ok, err := st2.tableHasColumn("credentials", "auth_cred_id")
	if err != nil || !ok {
		t.Fatalf("auth_cred_id missing: ok=%v err=%v", ok, err)
	}
	items, err := st2.ListCredentials("", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected wipe of legacy rows, got %d", len(items))
	}
}
