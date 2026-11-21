package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) ListProjectEnvs(projectID string) ([]model.ProjectEnv, error) {
	q := `SELECT id, project_id, env_id, ssh_cred_id, script, work_dir, last_status, last_deploy_at, created_at, updated_at FROM project_envs`
	var args []any
	if projectID != "" {
		q += ` WHERE project_id=?`
		args = append(args, projectID)
	}
	q += ` ORDER BY created_at`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ProjectEnv
	for rows.Next() {
		pe, err := scanProjectEnv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, pe)
	}
	return out, rows.Err()
}

func (s *Store) GetProjectEnv(id string) (model.ProjectEnv, error) {
	row := s.db.QueryRow(`SELECT id, project_id, env_id, ssh_cred_id, script, work_dir, last_status, last_deploy_at, created_at, updated_at FROM project_envs WHERE id=?`, id)
	pe, err := scanProjectEnv(row)
	if err == sql.ErrNoRows {
		return pe, ErrNotFound
	}
	return pe, err
}

func scanProjectEnv(row scanner) (model.ProjectEnv, error) {
	var pe model.ProjectEnv
	var lastDeploy sql.NullInt64
	var cAt, uAt int64
	err := row.Scan(&pe.ID, &pe.ProjectID, &pe.EnvID, &pe.SSHCredID, &pe.Script, &pe.WorkDir, &pe.LastStatus, &lastDeploy, &cAt, &uAt)
	if err != nil {
		return pe, err
	}
	pe.CreatedAt = time.Unix(cAt, 0)
	pe.UpdatedAt = time.Unix(uAt, 0)
	if lastDeploy.Valid {
		t := time.Unix(lastDeploy.Int64, 0)
		pe.LastDeployAt = &t
	}
	return pe, nil
}

func (s *Store) UpsertProjectEnv(in model.ProjectEnvInput) (model.ProjectEnv, error) {
	if in.ProjectID == "" || in.EnvID == "" {
		return model.ProjectEnv{}, fmtError("项目与环境不能为空")
	}
	if _, err := s.GetProject(in.ProjectID); err != nil {
		return model.ProjectEnv{}, err
	}
	if _, err := s.GetEnvironment(in.EnvID); err != nil {
		return model.ProjectEnv{}, err
	}
	var existingID string
	err := s.db.QueryRow(`SELECT id FROM project_envs WHERE project_id=? AND env_id=?`, in.ProjectID, in.EnvID).Scan(&existingID)
	now := nowUnix()
	if err == sql.ErrNoRows {
		pe := model.ProjectEnv{
			ID:        uuid.NewString(),
			ProjectID: in.ProjectID,
			EnvID:     in.EnvID,
			SSHCredID: strings.TrimSpace(in.SSHCredID),
			Script:    in.Script,
			WorkDir:   strings.TrimSpace(in.WorkDir),
			CreatedAt: time.Unix(now, 0),
			UpdatedAt: time.Unix(now, 0),
		}
		_, err = s.db.Exec(`INSERT INTO project_envs(id,project_id,env_id,ssh_cred_id,script,work_dir,last_status,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?)`,
			pe.ID, pe.ProjectID, pe.EnvID, pe.SSHCredID, pe.Script, pe.WorkDir, "", now, now)
		return pe, err
	}
	if err != nil {
		return model.ProjectEnv{}, err
	}
	_, err = s.db.Exec(`UPDATE project_envs SET ssh_cred_id=?, script=?, work_dir=?, updated_at=? WHERE id=?`,
		strings.TrimSpace(in.SSHCredID), in.Script, strings.TrimSpace(in.WorkDir), now, existingID)
	if err != nil {
		return model.ProjectEnv{}, err
	}
	return s.GetProjectEnv(existingID)
}

func (s *Store) DeleteProjectEnv(id string) error {
	res, err := s.db.Exec(`DELETE FROM project_envs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkProjectEnvDeploy(id, status string, at time.Time) error {
	_, err := s.db.Exec(`UPDATE project_envs SET last_status=?, last_deploy_at=?, updated_at=? WHERE id=?`,
		status, at.Unix(), nowUnix(), id)
	return err
}
