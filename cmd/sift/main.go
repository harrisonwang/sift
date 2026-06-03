// Command sift is a provider-driven tech-news aggregator: it discovers items
// from configured sources, caches them in SQLite, and renders reports.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	// Side-effect import: registers all provider types in the registry.
	_ "github.com/harrisonwang/sift/internal/provider/all"
)

// Build metadata, overridable via -ldflags by goreleaser.
var (
	version = "dev"
	commit  = ""
)

func main() {
	// Cancel work on SIGINT/SIGTERM so long fetches stop cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "sift: "+err.Error())
		os.Exit(1)
	}
}
