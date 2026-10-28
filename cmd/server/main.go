package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

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
	flag.Parse()

	logger, err := newLogger(*format, *level, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger.Info("server started", "addr", *addr, "data", *dir, "idle", *idle)
	err = http.ListenAndServe(*addr, newRouter(store.NewSQLite(*dir, *idle, logger), logger))
	logger.Error("server stopped", "err", err)
	os.Exit(1)
}
