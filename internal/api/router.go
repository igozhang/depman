package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"depman/internal/api/middleware"
	"depman/internal/api/read"
	"depman/internal/api/respond"
	"depman/internal/api/write"
	"depman/internal/auth"
	"depman/internal/cache"
	"depman/internal/config"
	"depman/internal/deploy"
	"depman/internal/store"
)

type UIConfig struct {
	EtcDir string
}

type Deps struct {
	Store   *store.Store
	Auth    *auth.Service
	Deploy  *deploy.Runner
	Cache   *cache.DiskStore
	WebFS   fs.FS
	Version string
	UI      UIConfig
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	if d.Auth == nil {
		d.Auth = auth.New("")
	}

	rh := &read.Handler{Store: d.Store, Auth: d.Auth}
	wh := &write.Handler{Store: d.Store, Runner: d.Deploy, Auth: d.Auth, Cache: d.Cache}

	r.GET("/api/health", func(c *gin.Context) {
		if d.Store != nil {
			if err := d.Store.Ping(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "version": d.Version, "error": "database"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": d.Version})
	})

	r.GET("/api/v1/read/ui", func(c *gin.Context) {
		ui := config.LoadUI(d.UI.EtcDir)
		respond.OK(c, gin.H{
			"title":     ui.Title,
			"tagline":   ui.Tagline,
			"hasBanner": ui.BannerPath != "",
			"bannerUrl": "/banner",
			"hasIcon":   true,
			"iconUrl":   "/icon",
			"version":   d.Version,
		})
	})

	r.GET("/banner", func(c *gin.Context) {
		ui := config.LoadUI(d.UI.EtcDir)
		if ui.BannerPath == "" {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Cache-Control", "public, max-age=30")
		c.File(ui.BannerPath)
	})

	serveIcon := func(c *gin.Context) {
		ui := config.LoadUI(d.UI.EtcDir)
		c.Header("Cache-Control", "public, max-age=30")
		if ui.IconPath != "" {
			c.File(ui.IconPath)
			return
		}
		c.Data(http.StatusOK, "image/svg+xml", config.DefaultIconBytes())
	}
	r.GET("/icon", serveIcon)
	r.GET("/favicon.ico", serveIcon)

	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.GET("/igokey", d.Auth.HandleIgoKey)
		authGroup.POST("/login", d.Auth.HandleLogin)
		authGroup.POST("/session", d.Auth.HandleSession)
	}

	readG := r.Group("/api/v1/read")
	{
		readG.GET("/projects", rh.ListCapsules)
		readG.GET("/projects/:id", rh.GetProject)
		readG.GET("/environments", rh.ListEnvironments)
		readG.GET("/tags", rh.ListTags)
		readG.GET("/credentials", rh.ListCredentials)
		readG.GET("/jobs", rh.ListJobs)
		readG.GET("/jobs/:id", rh.GetJob)
		readG.GET("/scoped-keys", middleware.AuthWrite(d.Auth), middleware.RequireSuper(), rh.ListScopedKeys)
	}

	writeG := r.Group("/api/v1/write", middleware.AuthWrite(d.Auth))
	{
		writeG.POST("/projects", wh.CreateProject)
		writeG.PUT("/projects/:id", wh.UpdateProject)
		writeG.DELETE("/projects/:id", wh.DeleteProject)

		writeG.POST("/environments", wh.CreateEnvironment)
		writeG.PUT("/environments/:id", wh.UpdateEnvironment)
		writeG.DELETE("/environments/:id", wh.DeleteEnvironment)

		writeG.POST("/credentials", wh.CreateCredential)
		writeG.PUT("/credentials/:id", wh.UpdateCredential)
		writeG.DELETE("/credentials/:id", wh.DeleteCredential)

		writeG.POST("/project-envs", wh.UpsertProjectEnv)
		writeG.DELETE("/project-envs/:id", wh.DeleteProjectEnv)

		writeG.POST("/deploy", wh.Deploy)
		writeG.POST("/ssh/test", wh.TestSSH)
		writeG.POST("/connectors/ping", wh.PingConnector)
		writeG.POST("/connectors/git/fetch", wh.GitFetch)
		writeG.POST("/connectors/registry/pull", wh.RegistryPull)
		writeG.POST("/connectors/registry/push", wh.RegistryPush)
		writeG.POST("/connectors/cluster/list", wh.ClusterList)
		writeG.POST("/connectors/cluster/namespaces", wh.ClusterNamespaces)
		writeG.POST("/connectors/cluster/workloads", wh.ClusterWorkloads)
		writeG.POST("/connectors/cluster/scale", wh.ClusterScale)
		writeG.POST("/connectors/cluster/restart", wh.ClusterRestart)

		writeG.POST("/scoped-keys", wh.CreateScopedKey)
		writeG.GET("/scoped-keys/:id/plain", wh.CopyScopedKey)
		writeG.DELETE("/scoped-keys/:id", wh.DeleteScopedKey)
	}

	if d.WebFS != nil {
		fileServer := http.FileServer(http.FS(d.WebFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") {
				respond.NotFound(c, "not found")
				return
			}
			if f, err := d.WebFS.Open(strings.TrimPrefix(path, "/")); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
