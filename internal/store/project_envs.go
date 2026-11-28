package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) ListProjectEnvs(projectID string) ([]model.ProjectEnv, error) {
	q := `SELECT id, project_id, env_id, name, ssh_cred_id, script, work_dir, last_status, last_deploy_at, deploy_count, created_at, updated_at FROM project_envs`
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
	row := s.db.QueryRow(`SELECT id, project_id, env_id, name, ssh_cred_id, script, work_dir, last_status, last_deploy_at, deploy_count, created_at, updated_at FROM project_envs WHERE id=?`, id)
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
	err := row.Scan(&pe.ID, &pe.ProjectID, &pe.EnvID, &pe.Name, &pe.SSHCredID, &pe.Script, &pe.WorkDir, &pe.LastStatus, &lastDeploy, &pe.DeployCount, &cAt, &uAt)
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
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.ProjectEnv{}, fmtError("发布任务名称不能为空")
	}
	if _, err := s.GetProject(in.ProjectID); err != nil {
		return model.ProjectEnv{}, err
	}
	if _, err := s.GetEnvironment(in.EnvID); err != nil {
		return model.ProjectEnv{}, err
	}
	cid := strings.TrimSpace(in.SSHCredID)
	if cid == "" {
		return model.ProjectEnv{}, fmtError("请选择 SSH 凭据")
	}
	cr, err := s.GetCredential(cid, false)
	if err == ErrNotFound {
		return model.ProjectEnv{}, fmtError("SSH 凭据不存在")
	}
	if err != nil {
		return model.ProjectEnv{}, err
	}
	if cr.Type != "ssh" {
		return model.ProjectEnv{}, fmtError("发布凭据须为 ssh 类型")
	}
	if cr.EnvID != in.EnvID {
		return model.ProjectEnv{}, fmtError("SSH 凭据须属于所选环境")
	}
	if strings.TrimSpace(in.Script) == "" {
		return model.ProjectEnv{}, fmtError("发布脚本不能为空")
	}
	now := nowUnix()
	id := strings.TrimSpace(in.ID)
	if id != "" {
		cur, err := s.GetProjectEnv(id)
		if err != nil {
			return cur, err
		}
		if cur.ProjectID != in.ProjectID {
			return cur, fmtError("不能更换所属项目")
		}
		_, err = s.db.Exec(`UPDATE project_envs SET env_id=?, name=?, ssh_cred_id=?, script=?, work_dir=?, updated_at=? WHERE id=?`,
			in.EnvID, name, cid, in.Script, strings.TrimSpace(in.WorkDir), now, id)
		if err != nil {
			return cur, err
		}
		return s.GetProjectEnv(id)
	}
	pe := model.ProjectEnv{
		ID:        uuid.NewString(),
		ProjectID: in.ProjectID,
		EnvID:     in.EnvID,
		Name:      name,
		SSHCredID: cid,
		Script:    in.Script,
		WorkDir:   strings.TrimSpace(in.WorkDir),
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO project_envs(id,project_id,env_id,name,ssh_cred_id,script,work_dir,last_status,deploy_count,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		pe.ID, pe.ProjectID, pe.EnvID, pe.Name, pe.SSHCredID, pe.Script, pe.WorkDir, "", 0, now, now)
	return pe, err
}

func (s *Store) DeleteProjectEnv(id string) error {
	_, _ = s.db.Exec(`DELETE FROM deploy_jobs WHERE project_env_id=?`, id)
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

// BumpDeployCount increments lifetime deploy counter for a publish task.
func (s *Store) BumpDeployCount(id string) error {
	_, err := s.db.Exec(`UPDATE project_envs SET deploy_count=deploy_count+1, updated_at=? WHERE id=?`, nowUnix(), id)
	return err
}
