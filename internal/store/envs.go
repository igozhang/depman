package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) ListEnvironments() ([]model.Environment, error) {
	rows, err := s.db.Query(`SELECT id, name, kind, description, created_at, updated_at FROM environments ORDER BY kind, name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Environment
	for rows.Next() {
		var e model.Environment
		var cAt, uAt int64
		if err := rows.Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &cAt, &uAt); err != nil {
			return nil, err
		}
		e.CreatedAt = time.Unix(cAt, 0)
		e.UpdatedAt = time.Unix(uAt, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetEnvironment(id string) (model.Environment, error) {
	var e model.Environment
	var cAt, uAt int64
	err := s.db.QueryRow(`SELECT id, name, kind, description, created_at, updated_at FROM environments WHERE id=?`, id).
		Scan(&e.ID, &e.Name, &e.Kind, &e.Description, &cAt, &uAt)
	if err == sql.ErrNoRows {
		return e, ErrNotFound
	}
	if err != nil {
		return e, err
	}
	e.CreatedAt = time.Unix(cAt, 0)
	e.UpdatedAt = time.Unix(uAt, 0)
	return e, nil
}

func normalizeKind(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "dev", "test", "staging", "prod", "other":
		return k
	default:
		if k == "" {
			return "other"
		}
		return k
	}
}

func (s *Store) CreateEnvironment(in model.EnvironmentInput) (model.Environment, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Environment{}, fmtError("环境名不能为空")
	}
	now := nowUnix()
	e := model.Environment{
		ID:          uuid.NewString(),
		Name:        name,
		Kind:        normalizeKind(in.Kind),
		Description: strings.TrimSpace(in.Description),
		CreatedAt:   time.Unix(now, 0),
		UpdatedAt:   time.Unix(now, 0),
	}
	_, err := s.db.Exec(`INSERT INTO environments(id,name,kind,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		e.ID, e.Name, e.Kind, e.Description, now, now)
	return e, err
}

func (s *Store) UpdateEnvironment(id string, in model.EnvironmentInput) (model.Environment, error) {
	e, err := s.GetEnvironment(id)
	if err != nil {
		return e, err
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		e.Name = n
	}
	if in.Kind != "" {
		e.Kind = normalizeKind(in.Kind)
	}
	e.Description = strings.TrimSpace(in.Description)
	now := nowUnix()
	e.UpdatedAt = time.Unix(now, 0)
	_, err = s.db.Exec(`UPDATE environments SET name=?, kind=?, description=?, updated_at=? WHERE id=?`,
		e.Name, e.Kind, e.Description, now, id)
	return e, err
}

func (s *Store) DeleteEnvironment(id string) error {
	res, err := s.db.Exec(`DELETE FROM environments WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
