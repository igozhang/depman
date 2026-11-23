package write

import (
	"strings"

	"github.com/gin-gonic/gin"

	"depman/internal/api/middleware"
	"depman/internal/api/respond"
	"depman/internal/auth"
	"depman/internal/deploy"
	"depman/internal/model"
	"depman/internal/sshx"
	"depman/internal/store"
)

type Handler struct {
	Store  *store.Store
	Runner *deploy.Runner
	Auth   *auth.Service
}

func (h *Handler) CreateProject(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.ProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	p, err := h.Store.CreateProject(in)
	if err != nil {
		if store.IsValidation(err) {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, p)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.ProjectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	p, err := h.Store.UpdateProject(c.Param("id"), in)
	if err == store.ErrNotFound {
		respond.NotFound(c, "项目不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, p)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	if err := h.Store.DeleteProject(c.Param("id")); err == store.ErrNotFound {
		respond.NotFound(c, "项目不存在")
		return
	} else if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) CreateEnvironment(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.EnvironmentInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	e, err := h.Store.CreateEnvironment(in)
	if err != nil {
		if store.IsValidation(err) {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, e)
}

func (h *Handler) UpdateEnvironment(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.EnvironmentInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	e, err := h.Store.UpdateEnvironment(c.Param("id"), in)
	if err == store.ErrNotFound {
		respond.NotFound(c, "环境不存在")
		return
	}
	if err != nil {
		if store.IsValidation(err) {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, e)
}

func (h *Handler) DeleteEnvironment(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	if err := h.Store.DeleteEnvironment(c.Param("id")); err == store.ErrNotFound {
		respond.NotFound(c, "环境不存在")
		return
	} else if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) CreateCredential(c *gin.Context) {
	info := middleware.Session(c)
	if !auth.IsSuper(info) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.CredentialInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cr, err := h.Store.CreateCredential(in)
	if err != nil {
		if store.IsValidation(err) || err == store.ErrNotFound {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, cr)
}

func (h *Handler) UpdateCredential(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.CredentialInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cr, err := h.Store.UpdateCredential(c.Param("id"), in)
	if err == store.ErrNotFound {
		respond.NotFound(c, "凭据不存在")
		return
	}
	if err != nil {
		if store.IsValidation(err) || err == store.ErrNotFound {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, cr)
}

func (h *Handler) DeleteCredential(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	if err := h.Store.DeleteCredential(c.Param("id")); err == store.ErrNotFound {
		respond.NotFound(c, "凭据不存在")
		return
	} else if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) UpsertProjectEnv(c *gin.Context) {
	info := middleware.Session(c)
	var in model.ProjectEnvInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	if !auth.CanAccessProject(info, in.ProjectID) || !auth.CanAccessEnv(info, in.EnvID) {
		respond.Forbidden(c, "无权操作该项目/环境")
		return
	}
	if !auth.IsSuper(info) {
		respond.Forbidden(c, "绑定发布配置需超管")
		return
	}
	pe, err := h.Store.UpsertProjectEnv(in)
	if err != nil {
		if store.IsValidation(err) || err == store.ErrNotFound {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, pe)
}

func (h *Handler) DeleteProjectEnv(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	if err := h.Store.DeleteProjectEnv(c.Param("id")); err == store.ErrNotFound {
		respond.NotFound(c, "不存在")
		return
	} else if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) Deploy(c *gin.Context) {
	info := middleware.Session(c)
	var in model.DeployInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	pe, err := h.Store.GetProjectEnv(in.ProjectEnvID)
	if err == store.ErrNotFound {
		respond.NotFound(c, "发布配置不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	if !auth.CanAccessProject(info, pe.ProjectID) || !auth.CanAccessEnv(info, pe.EnvID) {
		respond.Forbidden(c, "无权发布该项目/环境")
		return
	}
	if strings.TrimSpace(pe.Script) == "" {
		respond.BadRequest(c, "未配置发布脚本")
		return
	}
	if h.Runner == nil {
		respond.Internal(c, "deploy runner not configured")
		return
	}
	job, err := h.Runner.Enqueue(pe, info.Label)
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, job)
}

func (h *Handler) TestSSH(c *gin.Context) {
	info := middleware.Session(c)
	var body struct {
		CredID string `json:"credId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.CredID == "" {
		respond.BadRequest(c, "credId required")
		return
	}
	cred, secret, err := h.Store.PlainSecret(body.CredID)
	if err == store.ErrNotFound {
		respond.NotFound(c, "凭据不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	if cred.Type != "ssh" {
		respond.BadRequest(c, "仅 ssh 凭据可测试连通")
		return
	}
	if strings.TrimSpace(cred.Host) == "" {
		respond.BadRequest(c, "主机不能为空")
		return
	}
	if !auth.CanAccessEnv(info, cred.EnvID) {
		respond.Forbidden(c, "无权使用该凭据")
		return
	}
	t := sshx.Target{Host: cred.Host, Port: cred.Port, Username: cred.Username}
	if strings.Contains(secret, "BEGIN") && strings.Contains(secret, "PRIVATE KEY") {
		t.PrivateKey = secret
	} else {
		t.Password = secret
	}
	if err := sshx.TestConnect(t); err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) CreateScopedKey(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	var in model.ScopedKeyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	k, err := h.Store.CreateScopedKeyChecked(in, "super", h.Auth)
	if err != nil {
		if store.IsValidation(err) {
			respond.BadRequest(c, err.Error())
			return
		}
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, k)
}

func (h *Handler) DeleteScopedKey(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	if err := h.Store.DeleteScopedKey(c.Param("id")); err == store.ErrNotFound {
		respond.NotFound(c, "不存在")
		return
	} else if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) CopyScopedKey(c *gin.Context) {
	if !auth.IsSuper(middleware.Session(c)) {
		respond.Forbidden(c, "需要项目超管权限")
		return
	}
	plain, err := h.Store.PlainScopedKey(c.Param("id"))
	if err == store.ErrNotFound {
		respond.NotFound(c, "不可复制或已失效")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"plainKey": plain})
}
