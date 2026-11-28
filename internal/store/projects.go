package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func scanProject(row scanner) (model.Project, error) {
	var p model.Project
	var cAt, uAt int64
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.ParentID, &cAt, &uAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = time.Unix(cAt, 0)
	p.UpdatedAt = time.Unix(uAt, 0)
	return p, nil
}

func (s *Store) ListProjects() ([]model.Project, error) {
	rows, err := s.db.Query(`SELECT id, name, description, parent_id, created_at, updated_at FROM projects ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProject(id string) (model.Project, error) {
	row := s.db.QueryRow(`SELECT id, name, description, parent_id, created_at, updated_at FROM projects WHERE id=?`, id)
	p, err := scanProject(row)
	if err == sql.ErrNoRows {
		return p, ErrNotFound
	}
	return p, err
}

func (s *Store) ListChildProjects(parentID string) ([]model.Project, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, parent_id, created_at, updated_at FROM projects WHERE parent_id=? ORDER BY name COLLATE NOCASE`,
		parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProject(in model.ProjectInput) (model.Project, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Project{}, fmtError("项目名不能为空")
	}
	parentID := strings.TrimSpace(in.ParentID)
	if parentID != "" {
		parent, err := s.GetProject(parentID)
		if err != nil {
			if err == ErrNotFound {
				return model.Project{}, fmtError("父项目不存在")
			}
			return model.Project{}, err
		}
		if parent.ParentID != "" {
			return model.Project{}, fmtError("仅支持一级子项目，不可挂在子项目下")
		}
	}
	now := nowUnix()
	p := model.Project{
		ID:          uuid.NewString(),
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		ParentID:    parentID,
		CreatedAt:   time.Unix(now, 0),
		UpdatedAt:   time.Unix(now, 0),
	}
	_, err := s.db.Exec(`INSERT INTO projects(id,name,description,parent_id,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		p.ID, p.Name, p.Description, p.ParentID, now, now)
	return p, err
}

func (s *Store) UpdateProject(id string, in model.ProjectInput) (model.Project, error) {
	p, err := s.GetProject(id)
	if err != nil {
		return p, err
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		p.Name = n
	}
	p.Description = strings.TrimSpace(in.Description)
	// ParentID 创建后不可改，避免层级混乱
	now := nowUnix()
	p.UpdatedAt = time.Unix(now, 0)
	_, err = s.db.Exec(`UPDATE projects SET name=?, description=?, updated_at=? WHERE id=?`,
		p.Name, p.Description, now, id)
	return p, err
}

func (s *Store) DeleteProject(id string) error {
	// 先删子项目（及其绑定由 project_envs FK 级联）
	children, err := s.ListChildProjects(id)
	if err != nil {
		return err
	}
	for _, ch := range children {
		if err := s.DeleteProject(ch.ID); err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListProjectCapsules() ([]model.ProjectCapsule, error) {
	projects, err := s.ListProjects()
	if err != nil {
		return nil, err
	}
	out := make([]model.ProjectCapsule, 0, len(projects))
	for _, p := range projects {
		cap := model.ProjectCapsule{Project: p}
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM project_envs WHERE project_id=?`, p.ID).Scan(&cap.EnvCount)
		var status sql.NullString
		var deployAt sql.NullInt64
		var envName sql.NullString
		err := s.db.QueryRow(`
SELECT pe.last_status, pe.last_deploy_at, e.name
FROM project_envs pe
LEFT JOIN environments e ON e.id = pe.env_id
WHERE pe.project_id=? AND pe.last_deploy_at IS NOT NULL
ORDER BY pe.last_deploy_at DESC LIMIT 1`, p.ID).Scan(&status, &deployAt, &envName)
		if err == nil {
			if status.Valid {
				cap.LastStatus = status.String
			}
			if deployAt.Valid {
				t := time.Unix(deployAt.Int64, 0)
				cap.LastDeployAt = &t
			}
			if envName.Valid {
				cap.LastEnvName = envName.String
			}
		}
		out = append(out, cap)
	}
	return out, nil
}
