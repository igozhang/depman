package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) CreateDeployJob(projectID, envID, projectEnvID, script, createdBy string) (model.DeployJob, error) {
	now := time.Now()
	j := model.DeployJob{
		ID:           uuid.NewString(),
		ProjectID:    projectID,
		EnvID:        envID,
		ProjectEnvID: projectEnvID,
		Status:       "pending",
		Script:       script,
		CreatedAt:    now,
		CreatedBy:    createdBy,
	}
	_, err := s.db.Exec(`INSERT INTO deploy_jobs(id,project_id,env_id,project_env_id,status,script,created_at,created_by)
VALUES(?,?,?,?,?,?,?,?)`,
		j.ID, j.ProjectID, j.EnvID, j.ProjectEnvID, j.Status, j.Script, now.Unix(), createdBy)
	return j, err
}

func (s *Store) UpdateDeployJob(id, status, logTail, errMsg string, started, finished *time.Time) error {
	var sAt, fAt any
	if started != nil {
		sAt = started.Unix()
	}
	if finished != nil {
		fAt = finished.Unix()
	}
	_, err := s.db.Exec(`UPDATE deploy_jobs SET status=?, log_tail=?, error=?, started_at=COALESCE(?, started_at), finished_at=? WHERE id=?`,
		status, truncate(logTail, 64*1024), errMsg, sAt, fAt, id)
	return err
}

func (s *Store) GetDeployJob(id string) (model.DeployJob, error) {
	row := s.db.QueryRow(`SELECT id, project_id, env_id, project_env_id, status, script, log_tail, error, started_at, finished_at, created_at, created_by
FROM deploy_jobs WHERE id=?`, id)
	return scanJob(row)
}

func (s *Store) ListDeployJobs(limit int) ([]model.DeployJob, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, project_id, env_id, project_env_id, status, script, log_tail, error, started_at, finished_at, created_at, created_by
FROM deploy_jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DeployJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

const DeployHistoryKeep = 30

func (s *Store) ListDeployJobsByProjectEnv(projectEnvID string, limit int) ([]model.DeployJob, error) {
	if limit <= 0 {
		limit = DeployHistoryKeep
	}
	rows, err := s.db.Query(`SELECT id, project_id, env_id, project_env_id, status, script, log_tail, error, started_at, finished_at, created_at, created_by
FROM deploy_jobs WHERE project_env_id=? ORDER BY created_at DESC LIMIT ?`, projectEnvID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DeployJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// PruneDeployJobsKeepLatest deletes older jobs beyond keep, retaining only a count on project_envs.
func (s *Store) PruneDeployJobsKeepLatest(projectEnvID string, keep int) error {
	if keep <= 0 {
		keep = DeployHistoryKeep
	}
	rows, err := s.db.Query(`SELECT id FROM deploy_jobs WHERE project_env_id=? ORDER BY created_at DESC LIMIT -1 OFFSET ?`, projectEnvID, keep)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.db.Exec(`DELETE FROM deploy_jobs WHERE id=?`, id); err != nil {
			return err
		}
	}
	return nil
}

func scanJob(row scanner) (model.DeployJob, error) {
	var j model.DeployJob
	var sAt, fAt sql.NullInt64
	var cAt int64
	err := row.Scan(&j.ID, &j.ProjectID, &j.EnvID, &j.ProjectEnvID, &j.Status, &j.Script, &j.LogTail, &j.Error, &sAt, &fAt, &cAt, &j.CreatedBy)
	if err != nil {
		return j, err
	}
	j.CreatedAt = time.Unix(cAt, 0)
	if sAt.Valid {
		t := time.Unix(sAt.Int64, 0)
		j.StartedAt = &t
	}
	if fAt.Valid {
		t := time.Unix(fAt.Int64, 0)
		j.FinishedAt = &t
	}
	return j, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
