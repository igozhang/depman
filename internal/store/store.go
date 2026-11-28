package store

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"depman/internal/cred"
)

type Store struct {
	db  *sql.DB
	box *cred.Box
}

func Open(dbPath string, dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	box, err := cred.Open(dataDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db, box: box}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

func (s *Store) migrate() error {
	// Tables only — indexes that depend on newer columns come after schema ensure.
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS environments (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'dev',
  description TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS tags (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE COLLATE NOCASE,
  created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS environment_tags (
  env_id TEXT NOT NULL,
  tag_id TEXT NOT NULL,
  PRIMARY KEY(env_id, tag_id),
  FOREIGN KEY(env_id) REFERENCES environments(id) ON DELETE CASCADE,
  FOREIGN KEY(tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS credentials (
  id TEXT PRIMARY KEY,
  env_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  secret_enc TEXT NOT NULL DEFAULT '',
  extra TEXT NOT NULL DEFAULT '',
  auth_cred_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS project_envs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  env_id TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  ssh_cred_id TEXT NOT NULL DEFAULT '',
  script TEXT NOT NULL DEFAULT '',
  work_dir TEXT NOT NULL DEFAULT '',
  last_status TEXT NOT NULL DEFAULT '',
  last_deploy_at INTEGER,
  deploy_count INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY(env_id) REFERENCES environments(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS scoped_keys (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL UNIQUE,
  key_hint TEXT NOT NULL DEFAULT '',
  project_ids TEXT NOT NULL DEFAULT '[]',
  env_ids TEXT NOT NULL DEFAULT '[]',
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  plain_enc TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS deploy_jobs (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  env_id TEXT NOT NULL,
  project_env_id TEXT NOT NULL,
  status TEXT NOT NULL,
  script TEXT NOT NULL DEFAULT '',
  log_tail TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  started_at INTEGER,
  finished_at INTEGER,
  created_at INTEGER NOT NULL,
  created_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS host_resolves (
  id TEXT PRIMARY KEY,
  hostname TEXT NOT NULL UNIQUE COLLATE NOCASE,
  ip TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`)
	if err != nil {
		return err
	}
	if err := s.ensureCredentialSchema(); err != nil {
		return err
	}
	if err := s.ensureScopedKeysSchema(); err != nil {
		return err
	}
	_, err = s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_cred_env ON credentials(env_id);
CREATE INDEX IF NOT EXISTS idx_cred_auth ON credentials(auth_cred_id);
CREATE INDEX IF NOT EXISTS idx_pe_project ON project_envs(project_id);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON deploy_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_pe_created ON deploy_jobs(project_env_id, created_at DESC);
`)
	return err
}

func (s *Store) tableHasColumn(table, column string) (bool, error) {
	// pragma_table_info requires a literal table name; only call with fixed identifiers.
	var q string
	switch table {
	case "credentials":
		q = `SELECT COUNT(*) FROM pragma_table_info('credentials') WHERE name=?`
	case "scoped_keys":
		q = `SELECT COUNT(*) FROM pragma_table_info('scoped_keys') WHERE name=?`
	default:
		return false, fmtError("unknown table for schema check")
	}
	var n int
	err := s.db.QueryRow(q, column).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ensureCredentialSchema rebuilds credentials when the table predates auth_cred_id (dev wipe).
func (s *Store) ensureCredentialSchema() error {
	ok, err := s.tableHasColumn("credentials", "auth_cred_id")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	log.Printf("store: credentials schema outdated, recreating table (data discarded)")
	_, err = s.db.Exec(`
DROP TABLE IF EXISTS credentials;
CREATE TABLE credentials (
  id TEXT PRIMARY KEY,
  env_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  secret_enc TEXT NOT NULL DEFAULT '',
  extra TEXT NOT NULL DEFAULT '',
  auth_cred_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`)
	return err
}

// ensureScopedKeysSchema rebuilds scoped_keys when plain_enc is missing.
func (s *Store) ensureScopedKeysSchema() error {
	ok, err := s.tableHasColumn("scoped_keys", "plain_enc")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	log.Printf("store: scoped_keys schema outdated, recreating table (data discarded)")
	_, err = s.db.Exec(`
DROP TABLE IF EXISTS scoped_keys;
CREATE TABLE scoped_keys (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL UNIQUE,
  key_hint TEXT NOT NULL DEFAULT '',
  project_ids TEXT NOT NULL DEFAULT '[]',
  env_ids TEXT NOT NULL DEFAULT '[]',
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  created_by TEXT NOT NULL DEFAULT '',
  plain_enc TEXT NOT NULL DEFAULT ''
);
`)
	return err
}

func nowUnix() int64 { return time.Now().Unix() }
