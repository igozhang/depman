package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) ListCredentials(envID string, reveal bool) ([]model.Credential, error) {
	q := `SELECT id, env_id, name, type, host, port, username, secret_enc, extra, created_at, updated_at FROM credentials`
	var args []any
	if envID != "" {
		q += ` WHERE env_id=?`
		args = append(args, envID)
	}
	q += ` ORDER BY type, name COLLATE NOCASE`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Credential
	for rows.Next() {
		c, err := s.scanCredential(rows, reveal)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCredential(id string, reveal bool) (model.Credential, error) {
	row := s.db.QueryRow(`SELECT id, env_id, name, type, host, port, username, secret_enc, extra, created_at, updated_at FROM credentials WHERE id=?`, id)
	c, err := s.scanCredential(row, reveal)
	if err == sql.ErrNoRows {
		return c, ErrNotFound
	}
	return c, err
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanCredential(row scanner, reveal bool) (model.Credential, error) {
	var c model.Credential
	var secretEnc string
	var cAt, uAt int64
	err := row.Scan(&c.ID, &c.EnvID, &c.Name, &c.Type, &c.Host, &c.Port, &c.Username, &secretEnc, &c.Extra, &cAt, &uAt)
	if err != nil {
		return c, err
	}
	c.CreatedAt = time.Unix(cAt, 0)
	c.UpdatedAt = time.Unix(uAt, 0)
	c.SecretSet = secretEnc != ""
	if reveal && secretEnc != "" {
		plain, err := s.box.Open(secretEnc)
		if err != nil {
			return c, err
		}
		c.Secret = plain
	}
	return c, nil
}

func (s *Store) CreateCredential(in model.CredentialInput) (model.Credential, error) {
	name := strings.TrimSpace(in.Name)
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	if name == "" || in.EnvID == "" {
		return model.Credential{}, fmtError("名称与环境不能为空")
	}
	switch typ {
	case "git", "harbor", "ssh", "api":
	default:
		return model.Credential{}, fmtError("类型须为 git/harbor/ssh/api")
	}
	if _, err := s.GetEnvironment(in.EnvID); err != nil {
		return model.Credential{}, err
	}
	enc, err := s.box.Seal(in.Secret)
	if err != nil {
		return model.Credential{}, err
	}
	now := nowUnix()
	c := model.Credential{
		ID:        uuid.NewString(),
		EnvID:     in.EnvID,
		Name:      name,
		Type:      typ,
		Host:      strings.TrimSpace(in.Host),
		Port:      in.Port,
		Username:  strings.TrimSpace(in.Username),
		SecretSet: enc != "",
		Extra:     strings.TrimSpace(in.Extra),
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO credentials(id,env_id,name,type,host,port,username,secret_enc,extra,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.EnvID, c.Name, c.Type, c.Host, c.Port, c.Username, enc, c.Extra, now, now)
	return c, err
}

func (s *Store) UpdateCredential(id string, in model.CredentialInput) (model.Credential, error) {
	cur, err := s.GetCredential(id, false)
	if err != nil {
		return cur, err
	}
	if n := strings.TrimSpace(in.Name); n != "" {
		cur.Name = n
	}
	if t := strings.ToLower(strings.TrimSpace(in.Type)); t != "" {
		cur.Type = t
	}
	if in.EnvID != "" {
		cur.EnvID = in.EnvID
	}
	cur.Host = strings.TrimSpace(in.Host)
	if in.Port > 0 {
		cur.Port = in.Port
	}
	cur.Username = strings.TrimSpace(in.Username)
	cur.Extra = strings.TrimSpace(in.Extra)

	now := nowUnix()
	cur.UpdatedAt = time.Unix(now, 0)
	if in.Secret != "" {
		enc, err := s.box.Seal(in.Secret)
		if err != nil {
			return cur, err
		}
		cur.SecretSet = true
		_, err = s.db.Exec(`UPDATE credentials SET env_id=?, name=?, type=?, host=?, port=?, username=?, secret_enc=?, extra=?, updated_at=? WHERE id=?`,
			cur.EnvID, cur.Name, cur.Type, cur.Host, cur.Port, cur.Username, enc, cur.Extra, now, id)
		return cur, err
	}
	// 未传 secret 时保留原密文，避免误清空
	_, err = s.db.Exec(`UPDATE credentials SET env_id=?, name=?, type=?, host=?, port=?, username=?, extra=?, updated_at=? WHERE id=?`,
		cur.EnvID, cur.Name, cur.Type, cur.Host, cur.Port, cur.Username, cur.Extra, now, id)
	return cur, err
}

func (s *Store) DeleteCredential(id string) error {
	res, err := s.db.Exec(`DELETE FROM credentials WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PlainSecret returns decrypted secret for runtime use (SSH / Harbor).
func (s *Store) PlainSecret(id string) (model.Credential, string, error) {
	c, err := s.GetCredential(id, true)
	if err != nil {
		return c, "", err
	}
	return c, c.Secret, nil
}
