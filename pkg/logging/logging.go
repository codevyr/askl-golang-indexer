package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func Configure(level string) error {
	parsed, err := ParseLevel(level)
	if err != nil {
		return err
	}
	SetLevel(parsed)
	return nil
}

func ParseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", level)
	}
}

func SetLevel(level slog.Level) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func Debug(args ...any) {
	slog.Debug(fmt.Sprint(args...))
}

func Debugf(format string, args ...any) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func Info(args ...any) {
	slog.Info(fmt.Sprint(args...))
}

func Infof(format string, args ...any) {
	slog.Info(fmt.Sprintf(format, args...))
}

func Warn(args ...any) {
	slog.Warn(fmt.Sprint(args...))
}

func Warnf(format string, args ...any) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func Error(args ...any) {
	slog.Error(fmt.Sprint(args...))
}

func Errorf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
}

func Fatal(args ...any) {
	slog.Error(fmt.Sprint(args...))
	os.Exit(1)
}

func Fatalf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func Panic(args ...any) {
	message := fmt.Sprint(args...)
	slog.Error(message)
	panic(message)
}

func Panicf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	slog.Error(message)
	panic(message)
}
