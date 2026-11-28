package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
  env_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 0,
  username TEXT NOT NULL DEFAULT '',
  secret_enc TEXT NOT NULL DEFAULT '',
  extra TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(env_id) REFERENCES environments(id) ON DELETE CASCADE
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
  created_by TEXT NOT NULL DEFAULT ''
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
CREATE INDEX IF NOT EXISTS idx_cred_env ON credentials(env_id);
CREATE INDEX IF NOT EXISTS idx_pe_project ON project_envs(project_id);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON deploy_jobs(created_at DESC);
`)
	if err != nil {
		return err
	}
	// 兼容旧库：明文加密列（供超管复制）；已存在则忽略
	_, _ = s.db.Exec(`ALTER TABLE scoped_keys ADD COLUMN plain_enc TEXT NOT NULL DEFAULT ''`)
	// 兼容旧库：遗留 parent_id 列（已不再使用）
	_, _ = s.db.Exec(`ALTER TABLE projects ADD COLUMN parent_id TEXT NOT NULL DEFAULT ''`)
	// 兼容旧库：发布任务累计次数 + 按绑定查历史
	_, _ = s.db.Exec(`ALTER TABLE project_envs ADD COLUMN deploy_count INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.Exec(`ALTER TABLE project_envs ADD COLUMN name TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_jobs_pe_created ON deploy_jobs(project_env_id, created_at DESC)`)
	// 若尚未累计过，用现存 job 数回填（仅 deploy_count=0 时）
	_, _ = s.db.Exec(`UPDATE project_envs SET deploy_count=(
SELECT COUNT(*) FROM deploy_jobs WHERE deploy_jobs.project_env_id=project_envs.id
) WHERE deploy_count=0`)
	if err := s.migrateDropProjectEnvUnique(); err != nil {
		return err
	}
	// 旧数据无任务名时，用环境名回填；仍空则给占位名
	_, _ = s.db.Exec(`UPDATE project_envs SET name=(
SELECT e.name FROM environments e WHERE e.id=project_envs.env_id
) WHERE TRIM(name)=''`)
	_, _ = s.db.Exec(`UPDATE project_envs SET name='未命名任务' WHERE TRIM(name)=''`)
	return nil
}

// migrateDropProjectEnvUnique removes legacy UNIQUE(project_id, env_id) so one project can have many tasks per env.
func (s *Store) migrateDropProjectEnvUnique() error {
	var ddl string
	err := s.db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='project_envs'`).Scan(&ddl)
	if err == sql.ErrNoRows || ddl == "" {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.Contains(ddl, "UNIQUE(project_id, env_id)") && !strings.Contains(ddl, "UNIQUE (project_id, env_id)") {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(`
CREATE TABLE project_envs_new (
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
INSERT INTO project_envs_new(id,project_id,env_id,name,ssh_cred_id,script,work_dir,last_status,last_deploy_at,deploy_count,created_at,updated_at)
SELECT id,project_id,env_id,COALESCE(name,''),ssh_cred_id,script,work_dir,last_status,last_deploy_at,COALESCE(deploy_count,0),created_at,updated_at FROM project_envs;
DROP TABLE project_envs;
ALTER TABLE project_envs_new RENAME TO project_envs;
CREATE INDEX IF NOT EXISTS idx_pe_project ON project_envs(project_id);
`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func nowUnix() int64 { return time.Now().Unix() }
