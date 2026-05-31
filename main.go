// Command repoweaver starts the RepoWeaver local web application.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/mattcheramie/repoweaver/internal/config"
	"github.com/mattcheramie/repoweaver/internal/server"
	"github.com/mattcheramie/repoweaver/internal/store"
)

//go:embed web/templates/*.html web/static
var webFS embed.FS

func main() {
	cfg := config.Load()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	templatesFS, err := fs.Sub(webFS, "web/templates")
	if err != nil {
		log.Fatalf("templates fs: %v", err)
	}
	staticFS, err := fs.Sub(webFS, "web/static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}

	srv, err := server.New(cfg, st, templatesFS, staticFS)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	addr := ":" + cfg.Port
	url := "http://localhost:" + cfg.Port
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("RepoWeaver listening on %s  (LLM provider: %s, db: %s)", url, cfg.LLMProvider, cfg.DBPath)
	if cfg.OpenBrowser {
		go openBrowser(url)
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

// openBrowser tries to open the default browser at url. Best effort.
func openBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, cmd, args...).Start()
}
