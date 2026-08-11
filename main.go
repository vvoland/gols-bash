// SPDX-License-Identifier: GPL-3.0-only

// Command gols-bash runs the Bash language server over stdio.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"grono.dev/gols-bash/internal/server"
)

var (
	flagVersion  = flag.Bool("version", false, "print version and exit")
	flagLogFile  = flag.String("log-file", "", "write logs to file (default: stderr)")
	flagLogLevel = flag.String("log-level", "info", "log level: debug|info|warn|error")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "gols-bash — Bash language server (LSP over stdio)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  gols-bash [flags]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *flagVersion {
		fmt.Println(versionString())
		return
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(*flagLogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log-level: %v\n", err)
		os.Exit(2)
	}

	cfg := server.Config{
		In:      os.Stdin,
		Out:     os.Stdout,
		LogFile: *flagLogFile,
		Level:   lvl,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, cfg); errors.Is(err, server.ErrExitWithoutShutdown) {
		os.Exit(1)
	} else if err != nil {
		slog.With("error", err).ErrorContext(ctx, "server exited with error")
		os.Exit(1)
	}
}

func versionString() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "gols-bash (unknown)"
	}
	return fmt.Sprintf("gols-bash %s", info.Main.Version)
}
