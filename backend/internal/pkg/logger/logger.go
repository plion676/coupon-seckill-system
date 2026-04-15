package logger

import (
	"log/slog"
	"os"
	"strings"
)

func Init(service string) {
	level := parseLevel(os.Getenv("LOG_LEVEL"))

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := buildHandler(opts)
	slog.SetDefault(slog.New(handler).With("service", service))
}

func buildHandler(opts *slog.HandlerOptions) slog.Handler {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	if format == "text" {
		return slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.NewJSONHandler(os.Stdout, opts)
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
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
