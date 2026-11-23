package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/auth"
	"depman/internal/model"
)

func hashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func hintKey(plain string) string {
	if len(plain) < 4 {
		return "****"
	}
	return plain[:2] + "****" + plain[len(plain)-2:]
}

func (s *Store) LookupScopedKey(plain string) (ok bool, expiresAt time.Time, label string, projectIDs, envIDs []string) {
	plain = strings.TrimSpace(plain)
	if !looksLikeScoped(plain) {
		return false, time.Time{}, "", nil, nil
	}
	h := hashKey(plain)
	var exp int64
	var pJSON, eJSON string
	err := s.db.QueryRow(`SELECT label, project_ids, env_ids, expires_at FROM scoped_keys WHERE key_hash=?`, h).
		Scan(&label, &pJSON, &eJSON, &exp)
	if err != nil {
		return false, time.Time{}, "", nil, nil
	}
	expiresAt = time.Unix(exp, 0)
	if !expiresAt.After(time.Now()) {
		return false, time.Time{}, "", nil, nil
	}
	_ = json.Unmarshal([]byte(pJSON), &projectIDs)
	_ = json.Unmarshal([]byte(eJSON), &envIDs)
	if projectIDs == nil {
		projectIDs = []string{}
	}
	if envIDs == nil {
		envIDs = []string{}
	}
	return true, expiresAt, label, projectIDs, envIDs
}

func looksLikeScoped(key string) bool {
	if len(key) != auth.KeyLen {
		return false
	}
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

const scopedKeyRetain = 30

func (s *Store) ListScopedKeys() ([]model.ScopedKey, error) {
	_ = s.pruneScopedKeys()
	rows, err := s.db.Query(`SELECT id, label, key_hint, project_ids, env_ids, expires_at, created_at, created_by FROM scoped_keys WHERE expires_at > ? ORDER BY created_at DESC LIMIT ?`, time.Now().Unix(), scopedKeyRetain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ScopedKey
	for rows.Next() {
		var k model.ScopedKey
		var pJSON, eJSON string
		var exp, cAt int64
		if err := rows.Scan(&k.ID, &k.Label, &k.KeyHint, &pJSON, &eJSON, &exp, &cAt, &k.CreatedBy); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(pJSON), &k.ProjectIDs)
		_ = json.Unmarshal([]byte(eJSON), &k.EnvIDs)
		k.ExpiresAt = time.Unix(exp, 0)
		k.CreatedAt = time.Unix(cAt, 0)
		if k.ProjectIDs == nil {
			k.ProjectIDs = []string{}
		}
		if k.EnvIDs == nil {
			k.EnvIDs = []string{}
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// pruneScopedKeys 删除已过期，并只保留最近 scopedKeyRetain 条（超出直接注销）。
func (s *Store) pruneScopedKeys() error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(`DELETE FROM scoped_keys WHERE expires_at <= ?`, now); err != nil {
		return err
	}
	_, err := s.db.Exec(`
DELETE FROM scoped_keys WHERE id IN (
  SELECT id FROM scoped_keys ORDER BY created_at DESC LIMIT -1 OFFSET ?
)`, scopedKeyRetain)
	return err
}

// SiteKeyChecker reports whether a plain key is the active site super key.
type SiteKeyChecker interface {
	ValidateKey(key string) bool
}

func (s *Store) CreateScopedKey(in model.ScopedKeyInput, createdBy string) (model.ScopedKey, error) {
	return s.CreateScopedKeyChecked(in, createdBy, nil)
}

func (s *Store) CreateScopedKeyChecked(in model.ScopedKeyInput, createdBy string, site SiteKeyChecker) (model.ScopedKey, error) {
	ttl := in.TTLHours
	if ttl <= 0 {
		ttl = 24
	}
	if ttl > 168 {
		ttl = 168
	}
	pJSON, _ := json.Marshal(uniq(in.ProjectIDs))
	eJSON, _ := json.Marshal(uniq(in.EnvIDs))
	now := time.Now()
	exp := now.Add(time.Duration(ttl) * time.Hour)

	var plain string
	var err error
	okPlain := false
	for attempt := 0; attempt < 12; attempt++ {
		plain, err = auth.RandomPlainKey()
		if err != nil {
			return model.ScopedKey{}, err
		}
		if site != nil && site.ValidateKey(plain) {
			continue // 勿与当前超管 key 撞号，否则登录会被提升为超管
		}
		var exists int
		_ = s.db.QueryRow(`SELECT COUNT(1) FROM scoped_keys WHERE key_hash=?`, hashKey(plain)).Scan(&exists)
		if exists > 0 {
			continue
		}
		okPlain = true
		break
	}
	if !okPlain {
		return model.ScopedKey{}, fmtError("无法生成唯一 key")
	}

	enc, err := s.box.Seal(plain)
	if err != nil {
		return model.ScopedKey{}, err
	}
	k := model.ScopedKey{
		ID:         uuid.NewString(),
		Label:      strings.TrimSpace(in.Label),
		KeyHint:    hintKey(plain),
		ProjectIDs: uniq(in.ProjectIDs),
		EnvIDs:     uniq(in.EnvIDs),
		ExpiresAt:  exp,
		CreatedAt:  now,
		CreatedBy:  createdBy,
		PlainKey:   plain,
	}
	if k.Label == "" {
		k.Label = "项目管理"
	}
	_, err = s.db.Exec(`INSERT INTO scoped_keys(id,label,key_hash,key_hint,project_ids,env_ids,expires_at,created_at,created_by,plain_enc)
VALUES(?,?,?,?,?,?,?,?,?,?)`,
		k.ID, k.Label, hashKey(plain), k.KeyHint, string(pJSON), string(eJSON), exp.Unix(), now.Unix(), createdBy, enc)
	if err != nil {
		return model.ScopedKey{}, err
	}
	_ = s.pruneScopedKeys()
	return k, nil
}

// PlainScopedKey 解密出明文（仅超管复制用）；无密文或已失效则 NotFound。
func (s *Store) PlainScopedKey(id string) (string, error) {
	var enc string
	var exp int64
	err := s.db.QueryRow(`SELECT plain_enc, expires_at FROM scoped_keys WHERE id=?`, id).Scan(&enc, &exp)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if !time.Unix(exp, 0).After(time.Now()) || enc == "" {
		return "", ErrNotFound
	}
	return s.box.Open(enc)
}

func (s *Store) DeleteScopedKey(id string) error {
	res, err := s.db.Exec(`DELETE FROM scoped_keys WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetScopedKeyByID(id string) (model.ScopedKey, error) {
	var k model.ScopedKey
	var pJSON, eJSON string
	var exp, cAt int64
	err := s.db.QueryRow(`SELECT id, label, key_hint, project_ids, env_ids, expires_at, created_at, created_by FROM scoped_keys WHERE id=?`, id).
		Scan(&k.ID, &k.Label, &k.KeyHint, &pJSON, &eJSON, &exp, &cAt, &k.CreatedBy)
	if err == sql.ErrNoRows {
		return k, ErrNotFound
	}
	if err != nil {
		return k, err
	}
	_ = json.Unmarshal([]byte(pJSON), &k.ProjectIDs)
	_ = json.Unmarshal([]byte(eJSON), &k.EnvIDs)
	k.ExpiresAt = time.Unix(exp, 0)
	k.CreatedAt = time.Unix(cAt, 0)
	return k, nil
}

func uniq(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if out == nil {
		out = []string{}
	}
	return out
}
