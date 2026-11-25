package write

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"depman/internal/api/middleware"
	"depman/internal/api/respond"
	"depman/internal/appconn"
	"depman/internal/auth"
	"depman/internal/connector/k8sapi"
	"depman/internal/connector/rancher"
	"depman/internal/connector/registry"
	"depman/internal/model"
	"depman/internal/store"
)

func (h *Handler) credForWrite(c *gin.Context, credID, wantType string) (model.Credential, string, bool) {
	info := middleware.Session(c)
	if credID == "" {
		respond.BadRequest(c, "credId required")
		return model.Credential{}, "", false
	}
	cred, secret, err := h.Store.PlainSecret(credID)
	if err == store.ErrNotFound {
		respond.NotFound(c, "凭据不存在")
		return model.Credential{}, "", false
	}
	if err != nil {
		respond.Internal(c, err.Error())
		return model.Credential{}, "", false
	}
	if wantType != "" && cred.Type != wantType {
		respond.BadRequest(c, "凭据类型须为 "+wantType)
		return model.Credential{}, "", false
	}
	if !auth.CanAccessEnv(info, cred.EnvID) {
		respond.Forbidden(c, "无权使用该凭据")
		return model.Credential{}, "", false
	}
	if strings.TrimSpace(cred.Host) == "" {
		respond.BadRequest(c, "主机/地址不能为空")
		return model.Credential{}, "", false
	}
	return cred, secret, true
}

func (h *Handler) GitFetch(c *gin.Context) {
	var body struct {
		CredID string `json:"credId"`
		Ref    string `json:"ref"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cred, secret, ok := h.credForWrite(c, body.CredID, "git")
	if !ok {
		return
	}
	if h.Cache == nil {
		respond.Internal(c, "cache not configured")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()
	dir, err := appconn.GitFetch(ctx, h.Cache, cred, secret, body.Ref)
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true, "workDir": dir})
}

func (h *Handler) RegistryPull(c *gin.Context) {
	var body struct {
		CredID     string `json:"credId"`
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Digest     string `json:"digest"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cred, secret, ok := h.credForWrite(c, body.CredID, "harbor")
	if !ok {
		return
	}
	if h.Cache == nil {
		respond.Internal(c, "cache not configured")
		return
	}
	ref := registry.Ref{Repository: body.Repository, Tag: body.Tag, Digest: body.Digest}
	if strings.TrimSpace(ref.Repository) == "" {
		respond.BadRequest(c, "repository required")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Minute)
	defer cancel()
	path, key, digest, err := appconn.RegistryPull(ctx, h.Cache, cred, secret, ref)
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true, "path": path, "cacheKey": key, "digest": digest})
}

func (h *Handler) RegistryPush(c *gin.Context) {
	var body struct {
		CredID     string `json:"credId"`
		Repository string `json:"repository"`
		Tag        string `json:"tag"`
		Digest     string `json:"digest"`
		LocalPath  string `json:"localPath"`
		CacheKey   string `json:"cacheKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cred, secret, ok := h.credForWrite(c, body.CredID, "harbor")
	if !ok {
		return
	}
	ref := registry.Ref{Repository: body.Repository, Tag: body.Tag, Digest: body.Digest}
	if strings.TrimSpace(ref.Repository) == "" {
		respond.BadRequest(c, "repository required")
		return
	}
	local := strings.TrimSpace(body.LocalPath)
	if local == "" && body.CacheKey != "" && h.Cache != nil {
		if ent, found, err := h.Cache.Get(c.Request.Context(), body.CacheKey); err == nil && found {
			local = ent.Path
		}
	}
	if local == "" {
		respond.BadRequest(c, "localPath or cacheKey required")
		return
	}
	// Prevent path traversal: only allow files under the content cache root.
	if h.Cache == nil || !appconn.PathInCache(h.Cache, local) {
		respond.BadRequest(c, "localPath must be inside cache dir (prefer cacheKey)")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Minute)
	defer cancel()
	if err := appconn.RegistryPush(ctx, cred, secret, ref, local); err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) rancherFromReq(c *gin.Context, credID string) (*rancher.Client, bool) {
	cred, secret, ok := h.credForWrite(c, credID, "api")
	if !ok {
		return nil, false
	}
	cli, err := appconn.RancherClient(cred, secret)
	if err != nil {
		respond.BadRequest(c, err.Error())
		return nil, false
	}
	return cli, true
}

func (h *Handler) ClusterList(c *gin.Context) {
	var body struct {
		CredID string `json:"credId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cli, ok := h.rancherFromReq(c, body.CredID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	list, err := cli.ListClusters(ctx)
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true, "items": list})
}

func (h *Handler) ClusterNamespaces(c *gin.Context) {
	var body struct {
		CredID    string `json:"credId"`
		ClusterID string `json:"clusterId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cli, ok := h.rancherFromReq(c, body.CredID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	list, err := cli.ListNamespaces(ctx, body.ClusterID)
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true, "items": list})
}

func (h *Handler) ClusterWorkloads(c *gin.Context) {
	var body struct {
		CredID    string `json:"credId"`
		ClusterID string `json:"clusterId"`
		Namespace string `json:"namespace"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cli, ok := h.rancherFromReq(c, body.CredID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	list, err := cli.ListWorkloads(ctx, body.ClusterID, body.Namespace)
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true, "items": list})
}

func (h *Handler) ClusterScale(c *gin.Context) {
	var body struct {
		CredID    string `json:"credId"`
		ClusterID string `json:"clusterId"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
		Replicas  int32  `json:"replicas"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cli, ok := h.rancherFromReq(c, body.CredID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	err := cli.Scale(ctx, body.ClusterID, k8sapi.ScaleSpec{
		Ref:      k8sapi.ObjectRef{Namespace: body.Namespace, Name: body.Name, Kind: body.Kind},
		Replicas: body.Replicas,
	})
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true})
}

func (h *Handler) ClusterRestart(c *gin.Context) {
	var body struct {
		CredID    string `json:"credId"`
		ClusterID string `json:"clusterId"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Kind      string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		respond.BadRequest(c, "bad json")
		return
	}
	cli, ok := h.rancherFromReq(c, body.CredID)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	err := cli.Restart(ctx, body.ClusterID, k8sapi.ObjectRef{
		Namespace: body.Namespace,
		Name:      body.Name,
		Kind:      body.Kind,
	})
	if err != nil {
		respond.OK(c, gin.H{"ok": false, "error": err.Error()})
		return
	}
	respond.OK(c, gin.H{"ok": true})
}
