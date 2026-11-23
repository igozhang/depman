package store

import (
	"database/sql"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"depman/internal/model"
)

const (
	tagMaxRunes = 15 // 小于 16 个字符
	tagMaxCount = 30 // 全局标签上限
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
		tags, _ := s.tagsOfEnv(e.ID)
		e.Tags = tags
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
	tags, _ := s.tagsOfEnv(e.ID)
	e.Tags = tags
	return e, nil
}

func validateKind(k string) (string, error) {
	k = strings.ToLower(strings.TrimSpace(k))
	switch k {
	case "dev", "uat", "pro":
		return k, nil
	case "":
		return "dev", nil
	default:
		return "", fmtError("分类须为 dev / uat / pro")
	}
}

func (s *Store) CreateEnvironment(in model.EnvironmentInput) (model.Environment, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Environment{}, fmtError("环境名不能为空")
	}
	kind, err := validateKind(in.Kind)
	if err != nil {
		return model.Environment{}, err
	}
	now := nowUnix()
	e := model.Environment{
		ID:          uuid.NewString(),
		Name:        name,
		Kind:        kind,
		Description: strings.TrimSpace(in.Description),
		CreatedAt:   time.Unix(now, 0),
		UpdatedAt:   time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO environments(id,name,kind,description,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		e.ID, e.Name, e.Kind, e.Description, now, now)
	if err != nil {
		return e, err
	}
	tags, err := s.replaceEnvTags(e.ID, in.Tags)
	if err != nil {
		return e, err
	}
	e.Tags = tags
	_ = s.pruneTagsOverLimit()
	return e, nil
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
		kind, err := validateKind(in.Kind)
		if err != nil {
			return e, err
		}
		e.Kind = kind
	}
	e.Description = strings.TrimSpace(in.Description)
	now := nowUnix()
	e.UpdatedAt = time.Unix(now, 0)
	_, err = s.db.Exec(`UPDATE environments SET name=?, kind=?, description=?, updated_at=? WHERE id=?`,
		e.Name, e.Kind, e.Description, now, id)
	if err != nil {
		return e, err
	}
	tags, err := s.replaceEnvTags(id, in.Tags)
	if err != nil {
		return e, err
	}
	e.Tags = tags
	_ = s.pruneTagsOverLimit()
	return e, nil
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
	_ = s.pruneTagsOverLimit()
	return nil
}

func (s *Store) ListTags() ([]model.Tag, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM tags ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Tag
	for rows.Next() {
		var t model.Tag
		var cAt int64
		if err := rows.Scan(&t.ID, &t.Name, &cAt); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(cAt, 0)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) tagsOfEnv(envID string) ([]string, error) {
	rows, err := s.db.Query(`
SELECT t.name FROM tags t
INNER JOIN environment_tags et ON et.tag_id = t.id
WHERE et.env_id=?
ORDER BY t.name COLLATE NOCASE`, envID)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return out, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func normalizeTagName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmtError("标签不能为空")
	}
	if utf8.RuneCountInString(name) > tagMaxRunes {
		return "", fmtError("标签须小于 16 个字符")
	}
	return name, nil
}

func (s *Store) replaceEnvTags(envID string, names []string) ([]string, error) {
	seen := map[string]struct{}{}
	var clean []string
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		key := strings.ToLower(n)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if _, err := normalizeTagName(n); err != nil {
			return nil, err
		}
		clean = append(clean, n)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM environment_tags WHERE env_id=?`, envID); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(clean))
	for _, n := range clean {
		tagID, err := s.ensureTagTx(tx, n)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(`INSERT INTO environment_tags(env_id, tag_id) VALUES(?,?)`, envID, tagID); err != nil {
			return nil, err
		}
		var canonical string
		_ = tx.QueryRow(`SELECT name FROM tags WHERE id=?`, tagID).Scan(&canonical)
		if canonical == "" {
			canonical = n
		}
		out = append(out, canonical)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ensureTagTx(tx *sql.Tx, name string) (string, error) {
	name, err := normalizeTagName(name)
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(`SELECT id FROM tags WHERE name=? COLLATE NOCASE`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	if err := pruneUnusedTagsTx(tx, tagMaxCount-1); err != nil {
		return "", err
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n); err != nil {
		return "", err
	}
	if n >= tagMaxCount {
		return "", fmtError("标签总数已达上限 30")
	}
	id = uuid.NewString()
	_, err = tx.Exec(`INSERT INTO tags(id, name, created_at) VALUES(?,?,?)`, id, name, nowUnix())
	if err != nil {
		var existing string
		if e2 := tx.QueryRow(`SELECT id FROM tags WHERE name=? COLLATE NOCASE`, name).Scan(&existing); e2 == nil {
			return existing, nil
		}
		return "", err
	}
	return id, nil
}

// pruneTagsOverLimit 删除未使用标签，使总数 ≤ 30。
func (s *Store) pruneTagsOverLimit() error {
	return pruneUnusedTagsTx(s.db, tagMaxCount)
}

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// pruneUnusedTagsTx 在总数超过 keep 时，按创建时间从早到晚删除未引用标签。
func pruneUnusedTagsTx(q queryer, keep int) error {
	if keep < 0 {
		keep = 0
	}
	for {
		var n int
		if err := q.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n); err != nil {
			return err
		}
		if n <= keep {
			return nil
		}
		var id string
		err := q.QueryRow(`
SELECT t.id FROM tags t
LEFT JOIN environment_tags et ON et.tag_id = t.id
WHERE et.tag_id IS NULL
ORDER BY t.created_at ASC
LIMIT 1`).Scan(&id)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := q.Exec(`DELETE FROM tags WHERE id=?`, id); err != nil {
			return err
		}
	}
}
