package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

var userPassRegex = regexp.MustCompile(`(https?://)([^:]+):([^@]+)@`)

// MaskSensitive replaces plain-text passwords and basic auth credentials in URLs with asterisks.
func MaskSensitive(text string) string {
	return userPassRegex.ReplaceAllString(text, "${1}****:****@")
}

// Logger defines structured logging methods for the application.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// SlogLogger implements Logger wrapping standard library log/slog.
type SlogLogger struct {
	logger *slog.Logger
}

// NewLogger creates a new SlogLogger targeting out with specified severity level.
func NewLogger(out io.Writer, levelStr string) *SlogLogger {
	if out == nil {
		out = os.Stdout
	}

	level := parseLevel(levelStr)
	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Value.Kind() == slog.KindString {
				a.Value = slog.StringValue(MaskSensitive(a.Value.String()))
			}
			return a
		},
	}

	handler := slog.NewTextHandler(out, opts)
	return &SlogLogger{logger: slog.New(handler)}
}

// Debug logs at debug level.
func (l *SlogLogger) Debug(msg string, args ...any) {
	l.logger.Debug(MaskSensitive(msg), maskArgs(args...)...)
}

// Info logs at info level.
func (l *SlogLogger) Info(msg string, args ...any) {
	l.logger.Info(MaskSensitive(msg), maskArgs(args...)...)
}

// Warn logs at warn level.
func (l *SlogLogger) Warn(msg string, args ...any) {
	l.logger.Warn(MaskSensitive(msg), maskArgs(args...)...)
}

// Error logs at error level.
func (l *SlogLogger) Error(msg string, args ...any) {
	l.logger.Error(MaskSensitive(msg), maskArgs(args...)...)
}

// With returns a new Logger including key-value attributes.
func (l *SlogLogger) With(args ...any) Logger {
	return &SlogLogger{logger: l.logger.With(maskArgs(args...)...)}
}

func maskArgs(args ...any) []any {
	masked := make([]any, len(args))
	for i, arg := range args {
		if str, ok := arg.(string); ok {
			masked[i] = MaskSensitive(str)
		} else {
			masked[i] = arg
		}
	}
	return masked
}

func parseLevel(levelStr string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type nopLogger struct{}

// NopLogger returns a silent no-op Logger implementation for unit tests.
func NopLogger() Logger {
	return &nopLogger{}
}

func (n *nopLogger) Debug(msg string, args ...any) {}
func (n *nopLogger) Info(msg string, args ...any)  {}
func (n *nopLogger) Warn(msg string, args ...any)  {}
func (n *nopLogger) Error(msg string, args ...any) {}
func (n *nopLogger) With(args ...any) Logger       { return n }

// ContextWithLogger attaches logger to context.
func ContextWithLogger(ctx context.Context, log Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

// LoggerFromContext retrieves logger from context.
func LoggerFromContext(ctx context.Context) Logger {
	if log, ok := ctx.Value(loggerKey{}).(Logger); ok {
		return log
	}
	return NopLogger()
}

type loggerKey struct{}
