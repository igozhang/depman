package read

import (
	"strings"

	"github.com/gin-gonic/gin"

	"depman/internal/api/respond"
	"depman/internal/auth"
	"depman/internal/model"
	"depman/internal/store"
)

type Handler struct {
	Store *store.Store
	Auth  *auth.Service
}

func (h *Handler) optionalSession(c *gin.Context) (model.SessionInfo, bool) {
	tok := auth.TokenFromRequest(c)
	if tok == "" || h.Auth == nil {
		return model.SessionInfo{}, false
	}
	info, _, ok := h.Auth.SessionOf(tok)
	return info, ok
}

func (h *Handler) ListCapsules(c *gin.Context) {
	items, err := h.Store.ListProjectCapsules()
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, ok := h.optionalSession(c)
	if ok && info.Role == model.RoleManager {
		filtered := items[:0]
		for _, it := range items {
			if auth.CanAccessProject(info, it.ID) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) GetProject(c *gin.Context) {
	p, err := h.Store.GetProject(c.Param("id"))
	if err == store.ErrNotFound {
		respond.NotFound(c, "项目不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, authed := h.optionalSession(c)
	if authed && !auth.CanAccessProject(info, p.ID) {
		respond.Forbidden(c, "无权访问该项目")
		return
	}
	pes, _ := h.Store.ListProjectEnvs(p.ID)
	if authed && info.Role == model.RoleManager {
		filtered := pes[:0]
		for _, pe := range pes {
			if auth.CanAccessEnv(info, pe.EnvID) {
				filtered = append(filtered, pe)
			}
		}
		pes = filtered
	}
	respond.OK(c, gin.H{"project": p, "envs": pes})
}

func (h *Handler) ListEnvironments(c *gin.Context) {
	items, err := h.Store.ListEnvironments()
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, ok := h.optionalSession(c)
	if ok && info.Role == model.RoleManager {
		filtered := items[:0]
		for _, it := range items {
			if auth.CanAccessEnv(info, it.ID) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) ListTags(c *gin.Context) {
	items, err := h.Store.ListTags()
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) ListCredentials(c *gin.Context) {
	envID := strings.TrimSpace(c.Query("envId"))
	reveal := c.Query("reveal") == "1"
	info, authed := h.optionalSession(c)
	if reveal && !authed {
		respond.Unauthorized(c, "显示密钥需要登录")
		return
	}
	if reveal && !auth.IsSuper(info) {
		respond.Forbidden(c, "仅超管可明文查看密钥")
		return
	}
	items, err := h.Store.ListCredentials(envID, reveal)
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	if authed && info.Role == model.RoleManager {
		filtered := items[:0]
		for _, it := range items {
			if auth.CanAccessEnv(info, it.EnvID) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) ListHostResolves(c *gin.Context) {
	items, err := h.Store.ListHostResolves()
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	if items == nil {
		items = []model.HostResolve{}
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) ListProjectEnvJobs(c *gin.Context) {
	pe, err := h.Store.GetProjectEnv(c.Param("id"))
	if err == store.ErrNotFound {
		respond.NotFound(c, "发布任务不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, ok := h.optionalSession(c)
	if ok && (!auth.CanAccessProject(info, pe.ProjectID) || !auth.CanAccessEnv(info, pe.EnvID)) {
		respond.Forbidden(c, "无权查看该发布历史")
		return
	}
	items, err := h.Store.ListDeployJobsByProjectEnv(pe.ID, store.DeployHistoryKeep)
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	for i := range items {
		sanitizeJob(&items[i], info, ok)
	}
	respond.OK(c, gin.H{
		"items":      items,
		"totalCount": pe.DeployCount,
		"kept":       store.DeployHistoryKeep,
		"lastStatus": pe.LastStatus,
	})
}

func (h *Handler) ListJobs(c *gin.Context) {
	items, err := h.Store.ListDeployJobs(50)
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, ok := h.optionalSession(c)
	if ok && info.Role == model.RoleManager {
		filtered := items[:0]
		for _, it := range items {
			if auth.CanAccessProject(info, it.ProjectID) && auth.CanAccessEnv(info, it.EnvID) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	for i := range items {
		sanitizeJob(&items[i], info, ok)
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) GetJob(c *gin.Context) {
	j, err := h.Store.GetDeployJob(c.Param("id"))
	if err == store.ErrNotFound {
		respond.NotFound(c, "发布记录不存在")
		return
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	info, ok := h.optionalSession(c)
	if ok && (!auth.CanAccessProject(info, j.ProjectID) || !auth.CanAccessEnv(info, j.EnvID)) {
		respond.Forbidden(c, "无权查看该发布记录")
		return
	}
	sanitizeJob(&j, info, ok)
	respond.OK(c, j)
}

// sanitizeJob：无权限会话只保留状态摘要，避免只读面泄露脚本与命令输出。
func sanitizeJob(j *model.DeployJob, info model.SessionInfo, authed bool) {
	if authed && auth.IsSuper(info) {
		return
	}
	if authed && info.Role == model.RoleManager &&
		auth.CanAccessProject(info, j.ProjectID) && auth.CanAccessEnv(info, j.EnvID) {
		return
	}
	j.LogTail = ""
	j.Error = ""
	if j.Script != "" {
		j.Script = "（已隐藏）"
	}
}

func (h *Handler) ListScopedKeys(c *gin.Context) {
	items, err := h.Store.ListScopedKeys()
	if err != nil {
		respond.Internal(c, err.Error())
		return
	}
	respond.OK(c, gin.H{"items": items})
}
