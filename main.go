// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"grono.dev/gols-bash/internal/server"
)

func main() {
	var (
		logFile = flag.String("log", "", "if set, write server logs to this file instead of stderr")
		verbose = flag.Bool("verbose", false, "enable debug-level logging")
	)
	flag.Parse()

	cfg := server.Config{
		In:      os.Stdin,
		Out:     os.Stdout,
		LogFile: *logFile,
		Verbose: *verbose,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); err != nil {
		slog.With("error", err).ErrorContext(ctx, "server exited with error")
		os.Exit(1)
	}
}
