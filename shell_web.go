//go:build !desktop

package main

import (
	"net/http"

	"github.com/mattcheramie/repoweaver/internal/config"
)

// runShell is the default (headless) shell: it keeps the web server running and
// optionally opens the user's browser. It blocks until the process is killed.
func runShell(cfg config.Config, url string, _ *http.Server) {
	if cfg.OpenBrowser {
		go openBrowser(url)
	}
	select {} // block forever; the server runs in its own goroutine
}
