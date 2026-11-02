package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"prediction-market-matcher/config"
)

var globalLogger *slog.Logger

// InitLogger initializes the global structured logger based on configuration.
func InitLogger(cfg *config.Config) (*slog.Logger, error) {
	var logWriter io.Writer
	level := parseLogLevel(cfg.Log.Level)

	switch strings.ToLower(cfg.Log.Output) {
	case "stdout":
		logWriter = os.Stdout
	case "file":
		if err := os.MkdirAll(filepath.Dir(cfg.Log.File), 0755); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(cfg.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		logWriter = file
	case "both":
		if err := os.MkdirAll(filepath.Dir(cfg.Log.File), 0755); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(cfg.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		logWriter = io.MultiWriter(os.Stdout, file)
	default:
		logWriter = os.Stdout
	}

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && strings.ToLower(cfg.Log.Format) == "text" {
				return slog.Attr{}
			}
			return a
		},
		AddSource: cfg.DebugMode,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Log.Format) == "json" {
		handler = slog.NewJSONHandler(logWriter, opts)
	} else {
		handler = slog.NewTextHandler(logWriter, opts)
	}

	handler = handler.WithAttrs([]slog.Attr{
		slog.String("app", "events-matcher"),
		slog.String("version", "1.0.0"),
	})

	globalLogger = slog.New(handler)
	slog.SetDefault(globalLogger)

	return globalLogger, nil
}

// GetLogger returns the initialized global logger or falls back to a default stdout logger.
func GetLogger() *slog.Logger {
	if globalLogger == nil {
		globalLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(globalLogger)
	}
	return globalLogger
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Debug(msg string, args ...any) { GetLogger().Debug(msg, args...) }
func Info(msg string, args ...any)  { GetLogger().Info(msg, args...) }
func Warn(msg string, args ...any)  { GetLogger().Warn(msg, args...) }
func Error(msg string, args ...any) { GetLogger().Error(msg, args...) }

func LogError(err error, msg string, args ...any) {
	if err != nil {
		args = append(args, slog.String("error", err.Error()))
		GetLogger().Error(msg, args...)
	}
}

func WithRequestID(requestID string) *slog.Logger {
	return GetLogger().With(slog.String("request_id", requestID))
}

func DebugWithSource(msg string, args ...any) {
	if pc, file, line, ok := runtime.Caller(1); ok {
		funcName := runtime.FuncForPC(pc).Name()
		args = append(args,
			slog.String("source_file", file),
			slog.Int("source_line", line),
			slog.String("function", funcName),
		)
	}
	GetLogger().Debug(msg, args...)
}
