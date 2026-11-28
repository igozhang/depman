package store

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

// ListCredentials filters by envID:
//   - "" → all
//   - "shared" → env_id empty (public pool)
//   - otherwise → that environment
func (s *Store) ListCredentials(envID string, reveal bool) ([]model.Credential, error) {
	q := `SELECT id, env_id, name, type, host, port, username, secret_enc, extra, auth_cred_id, created_at, updated_at FROM credentials`
	var args []any
	switch envID {
	case "":
	case "shared":
		q += ` WHERE env_id=''`
	default:
		q += ` WHERE env_id=?`
		args = append(args, envID)
	}
	q += ` ORDER BY CASE WHEN env_id='' THEN 0 ELSE 1 END, type, name COLLATE NOCASE`
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
	row := s.db.QueryRow(`SELECT id, env_id, name, type, host, port, username, secret_enc, extra, auth_cred_id, created_at, updated_at FROM credentials WHERE id=?`, id)
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
	err := row.Scan(&c.ID, &c.EnvID, &c.Name, &c.Type, &c.Host, &c.Port, &c.Username, &secretEnc, &c.Extra, &c.AuthCredID, &cAt, &uAt)
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

func (s *Store) countAuthRefs(id string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM credentials WHERE auth_cred_id=?`, id).Scan(&n)
	return n, err
}

// assertAuthSourceMutable rejects changes that would break dependents.
func (s *Store) assertAuthSourceMutable(id, newType, newEnvID string) error {
	refs, err := s.countAuthRefs(id)
	if err != nil || refs == 0 {
		return err
	}
	cur, err := s.GetCredential(id, false)
	if err != nil {
		return err
	}
	if newType != cur.Type {
		return fmtError("该凭据已被引用，不能更改类型")
	}
	if cur.EnvID == "" && newEnvID != "" {
		return fmtError("该凭据已被引用，不能移出公共池")
	}
	if cur.EnvID != "" && newEnvID != cur.EnvID {
		return fmtError("该凭据已被引用，不能更换环境")
	}
	return nil
}

func (s *Store) validateAuthCred(selfID, authID, typ, envID string) error {
	authID = strings.TrimSpace(authID)
	if authID == "" {
		return nil
	}
	if envID == "" {
		return fmtError("公共凭据不能再引用其它认证")
	}
	if authID == selfID {
		return fmtError("认证凭据不能引用自身")
	}
	auth, err := s.GetCredential(authID, false)
	if err == ErrNotFound {
		return fmtError("认证凭据不存在")
	}
	if err != nil {
		return err
	}
	if auth.Type != typ {
		return fmtError("认证凭据类型须与当前一致")
	}
	if auth.AuthCredID != "" {
		return fmtError("认证凭据不能再引用其它凭据")
	}
	if auth.EnvID != "" && auth.EnvID != envID {
		return fmtError("只能引用公共凭据或同环境凭据")
	}
	return nil
}

func (s *Store) CreateCredential(in model.CredentialInput) (model.Credential, error) {
	name := strings.TrimSpace(in.Name)
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	envID := strings.TrimSpace(in.EnvID)
	authID := strings.TrimSpace(in.AuthCredID)
	host := strings.TrimSpace(in.Host)
	if name == "" {
		return model.Credential{}, fmtError("名称不能为空")
	}
	if host == "" {
		return model.Credential{}, fmtError("主机/地址不能为空")
	}
	switch typ {
	case "git", "harbor", "ssh", "api":
	default:
		return model.Credential{}, fmtError("类型须为 git/harbor/ssh/api")
	}
	if in.Port < 0 {
		return model.Credential{}, fmtError("端口不能为负数")
	}
	if envID != "" {
		if _, err := s.GetEnvironment(envID); err != nil {
			return model.Credential{}, err
		}
	}
	if err := s.validateAuthCred("", authID, typ, envID); err != nil {
		return model.Credential{}, err
	}
	if authID == "" && strings.TrimSpace(in.Secret) == "" {
		return model.Credential{}, fmtError("请填写密钥，或引用认证凭据")
	}
	enc, err := s.box.Seal(in.Secret)
	if err != nil {
		return model.Credential{}, err
	}
	now := nowUnix()
	c := model.Credential{
		ID:         uuid.NewString(),
		EnvID:      envID,
		Name:       name,
		Type:       typ,
		Host:       host,
		Port:       in.Port,
		Username:   strings.TrimSpace(in.Username),
		SecretSet:  enc != "",
		Extra:      strings.TrimSpace(in.Extra),
		AuthCredID: authID,
		CreatedAt:  time.Unix(now, 0),
		UpdatedAt:  time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO credentials(id,env_id,name,type,host,port,username,secret_enc,extra,auth_cred_id,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.EnvID, c.Name, c.Type, c.Host, c.Port, c.Username, enc, c.Extra, c.AuthCredID, now, now)
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
	newType := cur.Type
	if t := strings.ToLower(strings.TrimSpace(in.Type)); t != "" {
		switch t {
		case "git", "harbor", "ssh", "api":
			newType = t
		default:
			return cur, fmtError("类型须为 git/harbor/ssh/api")
		}
	}
	newEnvID := strings.TrimSpace(in.EnvID)
	if newEnvID != "" {
		if _, err := s.GetEnvironment(newEnvID); err != nil {
			return cur, err
		}
	}
	if err := s.assertAuthSourceMutable(id, newType, newEnvID); err != nil {
		return cur, err
	}
	cur.Type = newType
	cur.EnvID = newEnvID
	cur.Host = strings.TrimSpace(in.Host)
	if cur.Host == "" {
		return cur, fmtError("主机/地址不能为空")
	}
	cur.Port = in.Port
	if cur.Port < 0 {
		return cur, fmtError("端口不能为负数")
	}
	cur.Username = strings.TrimSpace(in.Username)
	cur.Extra = strings.TrimSpace(in.Extra)
	cur.AuthCredID = strings.TrimSpace(in.AuthCredID)
	if err := s.validateAuthCred(id, cur.AuthCredID, cur.Type, cur.EnvID); err != nil {
		return cur, err
	}
	if cur.AuthCredID == "" {
		// keep requiring a secret somehow: either already set or incoming
		if in.Secret == "" && !cur.SecretSet {
			return cur, fmtError("请填写密钥，或引用认证凭据")
		}
	}

	now := nowUnix()
	cur.UpdatedAt = time.Unix(now, 0)
	if in.Secret != "" {
		enc, err := s.box.Seal(in.Secret)
		if err != nil {
			return cur, err
		}
		cur.SecretSet = true
		_, err = s.db.Exec(`UPDATE credentials SET env_id=?, name=?, type=?, host=?, port=?, username=?, secret_enc=?, extra=?, auth_cred_id=?, updated_at=? WHERE id=?`,
			cur.EnvID, cur.Name, cur.Type, cur.Host, cur.Port, cur.Username, enc, cur.Extra, cur.AuthCredID, now, id)
		return cur, err
	}
	_, err = s.db.Exec(`UPDATE credentials SET env_id=?, name=?, type=?, host=?, port=?, username=?, extra=?, auth_cred_id=?, updated_at=? WHERE id=?`,
		cur.EnvID, cur.Name, cur.Type, cur.Host, cur.Port, cur.Username, cur.Extra, cur.AuthCredID, now, id)
	return cur, err
}

func (s *Store) DeleteCredential(id string) error {
	refs, err := s.countAuthRefs(id)
	if err != nil {
		return err
	}
	if refs > 0 {
		return fmtError("仍有凭据引用此认证，请先解除引用")
	}
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

func (s *Store) DeleteCredentialsByEnv(envID string) error {
	_, err := s.db.Exec(`DELETE FROM credentials WHERE env_id=?`, envID)
	return err
}

// PlainSecret returns decrypted secret for runtime use (SSH / Harbor).
func (s *Store) PlainSecret(id string) (model.Credential, string, error) {
	c, err := s.GetCredential(id, true)
	if err != nil {
		return c, "", err
	}
	return c, c.Secret, nil
}

// ResolveAuth returns the credential with username/secret taken from AuthCredID when set.
// Host/port/extra always come from the target credential.
func (s *Store) ResolveAuth(id string) (model.Credential, string, error) {
	c, secret, err := s.PlainSecret(id)
	if err != nil {
		return c, "", err
	}
	if c.AuthCredID == "" {
		return c, secret, nil
	}
	auth, authSecret, err := s.PlainSecret(c.AuthCredID)
	if err == ErrNotFound {
		return c, "", fmtError("认证凭据不存在")
	}
	if err != nil {
		return c, "", err
	}
	if auth.Type != c.Type {
		return c, "", fmtError("认证凭据类型与目标不一致")
	}
	if auth.AuthCredID != "" {
		return c, "", fmtError("认证凭据配置无效（嵌套引用）")
	}
	if auth.EnvID != "" && auth.EnvID != c.EnvID {
		return c, "", fmtError("认证凭据不属于公共池或同环境")
	}
	out := c
	if u := strings.TrimSpace(auth.Username); u != "" {
		out.Username = u
	}
	out.SecretSet = authSecret != "" || secret != ""
	if authSecret != "" {
		return out, authSecret, nil
	}
	if secret == "" {
		return out, "", fmtError("认证凭据未设置密钥")
	}
	return out, secret, nil
}
