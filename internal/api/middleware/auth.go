package middleware

import (
	"github.com/gin-gonic/gin"

	"depman/internal/api/respond"
	"depman/internal/auth"
	"depman/internal/model"
)

const (
	CtxSession = "depman.session"
)

// AuthWrite requires a valid session (X-Igo-Token or ?token=).
func AuthWrite(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if svc == nil {
			respond.Unauthorized(c, auth.FailMessage(auth.ReasonInvalid))
			c.Abort()
			return
		}
		tok := auth.TokenFromRequest(c)
		info, _, ok := svc.SessionOf(tok)
		if !ok {
			_, reason := svc.CheckToken(tok)
			respond.Unauthorized(c, auth.FailMessage(reason))
			c.Abort()
			return
		}
		c.Set(CtxSession, info)
		c.Next()
	}
}

func Session(c *gin.Context) model.SessionInfo {
	v, ok := c.Get(CtxSession)
	if !ok {
		return model.SessionInfo{}
	}
	info, _ := v.(model.SessionInfo)
	return info
}

// RequireSuper rejects non-super sessions.
func RequireSuper() gin.HandlerFunc {
	return func(c *gin.Context) {
		info := Session(c)
		if !auth.IsSuper(info) {
			respond.Forbidden(c, "需要项目超管权限")
			c.Abort()
			return
		}
		c.Next()
	}
}
