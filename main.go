// Command repoweaver starts the RepoWeaver application. By default it runs as a
// local web server (open the printed URL in a browser). Built with the
// "desktop" tag it instead opens a native OS window via a system webview that
// points at the same embedded server — see desktop.go.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net"
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

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

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

	// Bind explicitly so we know the final URL (including when PORT=0 picks a
	// free port, which the desktop shell relies on).
	ln, err := net.Listen("tcp", "127.0.0.1:"+cfg.Port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	url := "http://" + ln.Addr().String()

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	log.Printf("RepoWeaver %s ready on %s  (LLM: %s, analytics: %s, db: %s)",
		version, url, cfg.LLMProvider, cfg.AnalyticsProvider, cfg.DBPath)

	// runShell blocks until the app should exit. Its implementation is selected
	// by build tag: the default serves headlessly; the "desktop" build opens a
	// native window.
	runShell(cfg, url, httpServer)
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
