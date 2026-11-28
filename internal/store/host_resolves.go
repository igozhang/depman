package store

import (
	"database/sql"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"depman/internal/model"
)

func (s *Store) ListHostResolves() ([]model.HostResolve, error) {
	rows, err := s.db.Query(`SELECT id, hostname, ip, note, created_at, updated_at FROM host_resolves ORDER BY hostname COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.HostResolve
	for rows.Next() {
		h, err := scanHostResolve(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetHostResolve(id string) (model.HostResolve, error) {
	row := s.db.QueryRow(`SELECT id, hostname, ip, note, created_at, updated_at FROM host_resolves WHERE id=?`, id)
	h, err := scanHostResolve(row)
	if err == sql.ErrNoRows {
		return h, ErrNotFound
	}
	return h, err
}

func scanHostResolve(row scanner) (model.HostResolve, error) {
	var h model.HostResolve
	var cAt, uAt int64
	err := row.Scan(&h.ID, &h.Hostname, &h.IP, &h.Note, &cAt, &uAt)
	if err != nil {
		return h, err
	}
	h.CreatedAt = time.Unix(cAt, 0)
	h.UpdatedAt = time.Unix(uAt, 0)
	return h, nil
}

func normalizeHostResolve(in model.HostResolveInput) (hostname, ip, note string, err error) {
	hostname = strings.ToLower(strings.TrimSpace(in.Hostname))
	ip = strings.TrimSpace(in.IP)
	note = strings.TrimSpace(in.Note)
	if hostname == "" || ip == "" {
		return "", "", "", fmtError("主机名与 IP 不能为空")
	}
	if strings.ContainsAny(hostname, " \t/") || strings.Contains(hostname, "://") {
		return "", "", "", fmtError("主机名格式无效")
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		return "", "", "", fmtError("IP 地址无效")
	}
	if len(note) > 120 {
		return "", "", "", fmtError("备注最多 120 字")
	}
	return hostname, ip, note, nil
}

func (s *Store) CreateHostResolve(in model.HostResolveInput) (model.HostResolve, error) {
	hostname, ip, note, err := normalizeHostResolve(in)
	if err != nil {
		return model.HostResolve{}, err
	}
	now := nowUnix()
	h := model.HostResolve{
		ID:        uuid.NewString(),
		Hostname:  hostname,
		IP:        ip,
		Note:      note,
		CreatedAt: time.Unix(now, 0),
		UpdatedAt: time.Unix(now, 0),
	}
	_, err = s.db.Exec(`INSERT INTO host_resolves(id,hostname,ip,note,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		h.ID, h.Hostname, h.IP, h.Note, now, now)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.HostResolve{}, fmtError("该主机名已存在")
		}
		return model.HostResolve{}, err
	}
	return h, nil
}

func (s *Store) UpdateHostResolve(id string, in model.HostResolveInput) (model.HostResolve, error) {
	hostname, ip, note, err := normalizeHostResolve(in)
	if err != nil {
		return model.HostResolve{}, err
	}
	now := nowUnix()
	res, err := s.db.Exec(`UPDATE host_resolves SET hostname=?, ip=?, note=?, updated_at=? WHERE id=?`,
		hostname, ip, note, now, id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.HostResolve{}, fmtError("该主机名已存在")
		}
		return model.HostResolve{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.HostResolve{}, ErrNotFound
	}
	return s.GetHostResolve(id)
}

func (s *Store) DeleteHostResolve(id string) error {
	res, err := s.db.Exec(`DELETE FROM host_resolves WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
