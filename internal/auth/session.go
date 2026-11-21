package auth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"depman/internal/model"
)

const (
	ReasonKeyRevoked = "key_revoked"
	ReasonExpired    = "key_expired"
	ReasonInvalid    = "invalid_token"
)

type tokenState int

const (
	tokenOK tokenState = iota
	tokenMissing
	tokenExpired
	tokenKeyRevoked
)

func (s *Service) tokenStateOf(tok string) (tokenState, session) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return tokenMissing, session{}
	}
	now := time.Now()
	s.mu.Lock()
	s.syncActiveFromDiskLocked(now)
	s.mu.Unlock()

	s.sessMu.RLock()
	sess, ok := s.sessions[tok]
	s.sessMu.RUnlock()
	if !ok {
		return tokenKeyRevoked, session{}
	}
	if !sess.igoExpiresAt.After(now) {
		s.sessMu.Lock()
		delete(s.sessions, tok)
		s.sessMu.Unlock()
		return tokenExpired, sess
	}
	if sess.role == model.RoleSuper {
		if !s.ValidateKey(sess.igoKey) {
			s.sessMu.Lock()
			delete(s.sessions, tok)
			s.sessMu.Unlock()
			return tokenKeyRevoked, sess
		}
	} else if sess.role == model.RoleManager {
		if s.scoped == nil {
			s.sessMu.Lock()
			delete(s.sessions, tok)
			s.sessMu.Unlock()
			return tokenKeyRevoked, sess
		}
		ok, exp, _, _, _ := s.scoped.LookupScopedKey(sess.igoKey)
		if !ok || !exp.After(now) {
			s.sessMu.Lock()
			delete(s.sessions, tok)
			s.sessMu.Unlock()
			return tokenKeyRevoked, sess
		}
		sess.igoExpiresAt = exp
	}
	return tokenOK, sess
}

func (s *Service) CheckToken(tok string) (ok bool, reason string) {
	st, _ := s.tokenStateOf(tok)
	switch st {
	case tokenOK:
		return true, ""
	case tokenKeyRevoked:
		return false, ReasonKeyRevoked
	case tokenExpired:
		return false, ReasonExpired
	default:
		return false, ReasonInvalid
	}
}

func FailMessage(reason string) string {
	switch reason {
	case ReasonKeyRevoked:
		return "旧 key 不可用，请重新验证"
	case ReasonExpired:
		return "key 已过期，请重新验证"
	default:
		return "未认证，请先验证 key"
	}
}

func (s *Service) IssueSession(key string) (token string, expiresAt time.Time, info model.SessionInfo, ok bool) {
	key = strings.TrimSpace(key)
	s.mu.Lock()
	s.syncActiveFromDiskLocked(time.Now())
	s.mu.Unlock()

	if exp, valid := s.keyExpiry(key); valid {
		tok, err := newSessionToken()
		if err != nil {
			return "", time.Time{}, info, false
		}
		s.sessMu.Lock()
		s.pruneSessionsLocked(time.Now())
		s.sessions[tok] = session{
			igoKey:       key,
			igoExpiresAt: exp,
			role:         model.RoleSuper,
			label:        "项目超管",
		}
		s.sessMu.Unlock()
		info = model.SessionInfo{Role: model.RoleSuper, Label: "项目超管"}
		return tok, exp, info, true
	}

	if s.scoped != nil {
		valid, exp, label, pids, eids := s.scoped.LookupScopedKey(key)
		if valid {
			tok, err := newSessionToken()
			if err != nil {
				return "", time.Time{}, info, false
			}
			if label == "" {
				label = "项目管理"
			}
			s.sessMu.Lock()
			s.pruneSessionsLocked(time.Now())
			s.sessions[tok] = session{
				igoKey:       key,
				igoExpiresAt: exp,
				role:         model.RoleManager,
				label:        label,
				projectIDs:   append([]string{}, pids...),
				envIDs:       append([]string{}, eids...),
			}
			s.sessMu.Unlock()
			info = model.SessionInfo{
				Role:       model.RoleManager,
				Label:      label,
				ProjectIDs: pids,
				EnvIDs:     eids,
			}
			return tok, exp, info, true
		}
	}
	return "", time.Time{}, info, false
}

func (s *Service) SessionCheck(tok string) (expiresInSec int, info model.SessionInfo, reason string, ok bool) {
	st, sess := s.tokenStateOf(tok)
	if st == tokenOK {
		sec := int(time.Until(sess.igoExpiresAt).Seconds())
		if sec < 0 {
			sec = 0
		}
		info = model.SessionInfo{
			Role:       sess.role,
			Label:      sess.label,
			ProjectIDs: append([]string{}, sess.projectIDs...),
			EnvIDs:     append([]string{}, sess.envIDs...),
		}
		return sec, info, "", true
	}
	return 0, info, reasonFromState(st), false
}

func reasonFromState(st tokenState) string {
	switch st {
	case tokenKeyRevoked:
		return ReasonKeyRevoked
	case tokenExpired:
		return ReasonExpired
	default:
		return ReasonInvalid
	}
}

func (s *Service) SessionOf(tok string) (info model.SessionInfo, expiresAt time.Time, ok bool) {
	st, sess := s.tokenStateOf(tok)
	if st != tokenOK {
		return info, time.Time{}, false
	}
	info = model.SessionInfo{
		Role:       sess.role,
		Label:      sess.label,
		ProjectIDs: append([]string{}, sess.projectIDs...),
		EnvIDs:     append([]string{}, sess.envIDs...),
	}
	return info, sess.igoExpiresAt, true
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) pruneSessionsLocked(now time.Time) {
	for k, sess := range s.sessions {
		if !sess.igoExpiresAt.After(now) {
			delete(s.sessions, k)
		}
	}
}

func (s *Service) revokeSessionsForKey(igoKey string) {
	if igoKey == "" {
		return
	}
	now := time.Now()
	s.sessMu.Lock()
	for tok, sess := range s.sessions {
		if sess.igoKey == igoKey {
			delete(s.sessions, tok)
		}
	}
	s.pruneSessionsLocked(now)
	s.sessMu.Unlock()
}

// CanAccessProject returns whether the session may see/operate the project.
func CanAccessProject(info model.SessionInfo, projectID string) bool {
	if info.Role == model.RoleSuper {
		return true
	}
	if info.Role != model.RoleManager {
		return false
	}
	if len(info.ProjectIDs) == 0 {
		return true // empty means all projects within env scope
	}
	for _, id := range info.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

func CanAccessEnv(info model.SessionInfo, envID string) bool {
	if info.Role == model.RoleSuper {
		return true
	}
	if info.Role != model.RoleManager {
		return false
	}
	if len(info.EnvIDs) == 0 {
		return true
	}
	for _, id := range info.EnvIDs {
		if id == envID {
			return true
		}
	}
	return false
}

func IsSuper(info model.SessionInfo) bool {
	return info.Role == model.RoleSuper
}
