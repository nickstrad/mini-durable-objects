package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nickstrad/mini-durable-objects/internal/events"
	"github.com/nickstrad/mini-durable-objects/internal/store"
)

var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// newLogger builds the server logger. format is "text" or "json" (JSONL).
// level is "debug", "info", "warn", or "error".
func newLogger(format, level string, w io.Writer) (*slog.Logger, error) {
	lvl, ok := logLevels[level]
	if !ok {
		return nil, fmt.Errorf("unknown -log-level %q (want debug, info, warn, or error)", level)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	}
	return nil, fmt.Errorf("unknown -log-format %q (want text or json)", format)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	dir := flag.String("data", "data/objects", "directory for object databases")
	idle := flag.Duration("idle", time.Minute, "close an object's database after it is unused this long")
	format := flag.String("log-format", "text", "log format: text or json")
	level := flag.String("log-level", "info", "log level: debug, info, warn, or error")
	eventsDir := flag.String("events", "", "directory for event CSV files (empty: events off)")
	eventsFlush := flag.Duration("events-flush", time.Second, "time between flushes of the event files")
	flag.Parse()

	logger, err := newLogger(*format, *level, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var rec *events.Recorder // nil: events off
	if *eventsDir != "" {
		if rec, err = events.Open(*eventsDir, *eventsFlush, logger); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	logger.Info("server started", "addr", *addr, "data", *dir, "idle", *idle)
	srv := &http.Server{Addr: *addr, Handler: newRouter(store.NewSQLite(*dir, *idle, logger, rec), logger)}
	os.Exit(serve(srv, rec, logger))
}

// serve runs srv until it fails or the process gets SIGINT or SIGTERM. Then it
// stops the HTTP server and closes rec, so the events in the buffer reach the
// files. It returns the exit status.
func serve(srv *http.Server, rec *events.Recorder, logger *slog.Logger) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failed := make(chan error, 1)
	go func() { failed <- srv.ListenAndServe() }()

	status := 0
	select {
	case err := <-failed:
		logger.Error("server stopped", "err", err)
		status = 1
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Warn("server shutdown incomplete", "err", err)
		}
	}
	if err := rec.Close(); err != nil {
		logger.Error("events close failed", "err", err)
	}
	if status == 0 {
		logger.Info("server stopped")
	}
	return status
}
