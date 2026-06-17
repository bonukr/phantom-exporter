// Package logging configures a slog-based logger that writes structured logs to a file.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// New creates a JSON slog.Logger that writes to the given file (parent dirs are
// created as needed) and mirrors to stdout for convenient local debugging. If
// the log file cannot be opened, it falls back to stdout-only so the service
// can still start. The returned closer should be called on shutdown.
func New(logFile, level string) (*slog.Logger, io.Closer, error) {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	w, closer := stdoutOnly()
	if f, err := openLogFile(logFile); err == nil {
		w = io.MultiWriter(os.Stdout, f)
		closer = f
	} else {
		defer func() {
			slog.New(slog.NewJSONHandler(os.Stdout, opts)).
				Warn("log file unavailable, using stdout only", "file", logFile, "error", err.Error())
		}()
	}

	return slog.New(slog.NewJSONHandler(w, opts)), closer, nil
}

func openLogFile(logFile string) (*os.File, error) {
	if dir := filepath.Dir(logFile); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log dir: %w", err)
		}
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return f, nil
}

func stdoutOnly() (io.Writer, io.Closer) {
	return os.Stdout, io.NopCloser(nil)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
