package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"depman/internal/api"
	"depman/internal/appconn"
	"depman/internal/auth"
	"depman/internal/config"
	"depman/internal/deploy"
	"depman/internal/store"
)

//go:embed all:webdist
var webEmbed embed.FS

var version = "0.1.0"

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}

	st, err := store.Open(cfg.DBPath, cfg.DataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	authSvc := auth.New(cfg.DataDir)
	authSvc.SetScopedChecker(st)

	cacheStore, err := appconn.OpenCache(cfg.CacheDir)
	if err != nil {
		log.Fatalf("open cache: %v", err)
	}
	runCacheGC := func() {
		if n, freed, err := cacheStore.GC(context.Background(), time.Time{}); err != nil {
			log.Printf("cache gc: %v", err)
		} else if n > 0 {
			log.Printf("cache gc: removed %d objects, freed %d bytes", n, freed)
		}
	}
	runCacheGC()
	cacheGCStop := make(chan struct{})
	go func() {
		t := time.NewTicker(6 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				runCacheGC()
			case <-cacheGCStop:
				return
			}
		}
	}()
	defer close(cacheGCStop)

	runner := &deploy.Runner{Store: st}
	log.Printf("cache ready at %s", cacheStore.Root())

	var webFS fs.FS
	sub, err := fs.Sub(webEmbed, "webdist")
	if err == nil {
		if f, err := sub.Open("index.html"); err == nil {
			_ = f.Close()
			webFS = sub
		}
	}

	r := api.NewRouter(api.Deps{
		Store:   st,
		Auth:    authSvc,
		Deploy:  runner,
		Cache:   cacheStore,
		WebFS:   webFS,
		Version: version,
		UI:      api.UIConfig{EtcDir: cfg.EtcDir},
	})

	ui := config.LoadUI(cfg.EtcDir)
	iconNote := "builtin-cicd-svg"
	if ui.IconPath != "" {
		iconNote = ui.IconPath
	}
	log.Printf("depman %s listening on %s (db=%s etc=%s icon=%s)", version, cfg.Addr, cfg.DBPath, cfg.EtcDir, iconNote)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Println(err)
			os.Exit(1)
		}
	case sig := <-sigCh:
		log.Printf("signal %v, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}
