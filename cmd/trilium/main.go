// Command trilium works with Trilium Notes through its ETAPI.
package main

import (
	"context"
	"os"
	"os/signal"
	"time"

	"github.com/oyasmi/trilium-cli/internal/cli"
)

// Set by the build (see Makefile and .goreleaser.yaml).
var (
	version = "dev"
	commit  = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	rt := &cli.Runtime{
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Getenv:    os.Getenv,
		Now:       time.Now,
		Loc:       time.Local,
		Version:   version,
		Commit:    commit,
		WriteFile: func(name string, data []byte) error { return os.WriteFile(name, data, 0o644) },
	}
	code := cli.Execute(ctx, rt, os.Args[1:])
	stop()
	os.Exit(code)
}
