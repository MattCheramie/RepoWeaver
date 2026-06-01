//go:build desktop

// This file is compiled only with `-tags desktop`. It wraps the embedded web
// server in a native OS window using a system webview, giving RepoWeaver a
// lightweight desktop app shell (per the README's "Wails / Webview" goal)
// without changing any backend code.
//
// Build requirements (not needed for the default web build):
//   - CGO enabled (CGO_ENABLED=1) and a C toolchain.
//   - A system webview: Linux needs libwebkit2gtk-4.1-dev (or 4.0) and GTK;
//     macOS uses WebKit (built in); Windows uses the Edge WebView2 runtime.
//   - A graphical display at runtime.
//
// Example:
//   CGO_ENABLED=1 go build -tags desktop -o bin/repoweaver-desktop .

package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/mattcheramie/repoweaver/internal/config"
	webview "github.com/webview/webview_go"
)

// runShell opens a native window pointed at the local server and shuts the
// server down cleanly when the window closes.
func runShell(_ config.Config, url string, httpServer *http.Server) {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("RepoWeaver")
	w.SetSize(1200, 850, webview.HintNone)
	w.Navigate(url)
	w.Run() // blocks until the window is closed

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
