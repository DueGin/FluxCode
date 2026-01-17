package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	setupOnce sync.Once
	base      *slog.Logger
)

// Setup initializes slog as the default logger and bridges standard log output.
func Setup() {
	setupOnce.Do(func() {
		level := parseLevel(os.Getenv("LOG_LEVEL"))
		handler := slog.NewTextHandler(levelStrippingWriter{w: os.Stdout}, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: replaceTimeAttr,
		})
		base = slog.New(handler)
		slog.SetDefault(base)

		log.SetFlags(0)
		log.SetPrefix("")
		log.SetOutput(stdWriter{})
	})
}

func logger() *slog.Logger {
	if base != nil {
		return base
	}
	return slog.Default()
}

// Printf logs with an inferred level (keeps legacy log.Printf call sites).
func Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logWithLevel(inferLevel(msg), msg)
}

// Println logs with an inferred level (keeps legacy log.Println call sites).
func Println(args ...any) {
	msg := fmt.Sprint(args...)
	logWithLevel(inferLevel(msg), msg)
}

// Print logs with an inferred level (keeps legacy log.Print call sites).
func Print(args ...any) {
	msg := fmt.Sprint(args...)
	logWithLevel(inferLevel(msg), msg)
}

// Infof logs an info-level message.
func Infof(format string, args ...any) {
	logWithLevel(slog.LevelInfo, fmt.Sprintf(format, args...))
}

// Warnf logs a warn-level message.
func Warnf(format string, args ...any) {
	logWithLevel(slog.LevelWarn, fmt.Sprintf(format, args...))
}

// Errorf logs an error-level message.
func Errorf(format string, args ...any) {
	logWithLevel(slog.LevelError, fmt.Sprintf(format, args...))
}

// Debugf logs a debug-level message.
func Debugf(format string, args ...any) {
	logWithLevel(slog.LevelDebug, fmt.Sprintf(format, args...))
}

// Info logs an info-level message.
func Info(args ...any) {
	logWithLevel(slog.LevelInfo, fmt.Sprint(args...))
}

// Warn logs a warn-level message.
func Warn(args ...any) {
	logWithLevel(slog.LevelWarn, fmt.Sprint(args...))
}

// Error logs an error-level message.
func Error(args ...any) {
	logWithLevel(slog.LevelError, fmt.Sprint(args...))
}

// Debug logs a debug-level message.
func Debug(args ...any) {
	logWithLevel(slog.LevelDebug, fmt.Sprint(args...))
}

// Fatalf logs an error-level message and exits.
func Fatalf(format string, args ...any) {
	logWithLevel(slog.LevelError, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Fatal logs an error-level message and exits.
func Fatal(args ...any) {
	logWithLevel(slog.LevelError, fmt.Sprint(args...))
	os.Exit(1)
}

func logWithLevel(level slog.Level, msg string) {
	if msg == "" {
		return
	}
	logger().Log(context.Background(), level, msg)
}

type stdWriter struct{}

func (stdWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	logWithLevel(inferLevel(msg), msg)
	return len(p), nil
}

type levelStrippingWriter struct {
	w io.Writer
}

func (lw levelStrippingWriter) Write(p []byte) (n int, err error) {
	cleaned := stripStructuredKeys(string(p))
	return lw.w.Write([]byte(cleaned))
}

func stripStructuredKeys(line string) string {
	base, suffix := splitLineSuffix(line)
	tokens := splitTokensPreserveQuotes(base)
	if len(tokens) == 0 {
		return line
	}

	for i, tok := range tokens {
		tok = stripTokenKey(tok, "time=")
		tok = stripTokenKey(tok, "level=")
		tok = stripTokenKey(tok, "msg=")
		tok = maybeUnquote(tok)
		tokens[i] = tok
	}

	return strings.Join(tokens, " ") + suffix
}

func splitLineSuffix(line string) (string, string) {
	if line == "" {
		return "", ""
	}
	end := len(line)
	for end > 0 {
		last := line[end-1]
		if last == '\n' || last == '\r' {
			end--
			continue
		}
		break
	}
	return line[:end], line[end:]
}

func stripTokenKey(token, key string) string {
	if strings.HasPrefix(token, key) {
		return token[len(key):]
	}
	return token
}

func splitTokensPreserveQuotes(s string) []string {
	var tokens []string
	var buf strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range s {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			buf.WriteRune(r)
			continue
		}

		if r == '"' {
			inQuotes = !inQuotes
			buf.WriteRune(r)
			continue
		}

		if (r == ' ' || r == '\t' || r == '\n') && !inQuotes {
			if buf.Len() > 0 {
				tokens = append(tokens, buf.String())
				buf.Reset()
			}
			continue
		}

		buf.WriteRune(r)
	}

	if buf.Len() > 0 {
		tokens = append(tokens, buf.String())
	}

	return tokens
}

func replaceTimeAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		attr.Value = slog.StringValue(attr.Value.Time().Format("2006-01-02 15:04:05"))
	}
	return attr
}

func maybeUnquote(token string) string {
	if len(token) < 2 || token[0] != '"' || token[len(token)-1] != '"' {
		return token
	}
	unquoted, err := strconv.Unquote(token)
	if err != nil {
		return token[1 : len(token)-1]
	}
	return unquoted
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

func inferLevel(msg string) slog.Level {
	trimmed := strings.TrimSpace(msg)
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasPrefix(lower, "[debug]"),
		strings.HasPrefix(lower, "debug:"),
		strings.HasPrefix(lower, "debug "):
		return slog.LevelDebug
	case strings.HasPrefix(lower, "[warn]"),
		strings.HasPrefix(lower, "warn:"),
		strings.HasPrefix(lower, "warning:"):
		return slog.LevelWarn
	case strings.HasPrefix(lower, "[error]"),
		strings.HasPrefix(lower, "error:"):
		return slog.LevelError
	}

	if strings.Contains(lower, "failed") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "fatal") {
		return slog.LevelError
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") {
		return slog.LevelWarn
	}
	if strings.Contains(lower, "debug") {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
