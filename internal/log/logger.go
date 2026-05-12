// Package log provides the CLI-wide structured logger used by debug mode.
//
// Logging is intentionally off-by-default: the zero state of the package
// emits nothing, so normal command output stays clean. Callers opt in by
// passing --debug / -v / -vv on the command line, or by exporting
// AONE_DEBUG / AONE_LOG_LEVEL / AONE_LOG_FORMAT / AONE_LOG_FILE.
//
// All logs are written to stderr (or AONE_LOG_FILE when set) so they never
// pollute stdout — keeping pipelines like `aone sandbox list -f json | jq`
// fully functional regardless of the active log level.
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// LevelTrace is the most verbose level, below slog.LevelDebug. It enables
// full request/response dumps (headers + body, after redaction) on top of
// everything LevelDebug already emits.
const LevelTrace slog.Level = -8

// Level represents the resolved verbosity for the current invocation.
type Level slog.Level

// String returns a human-readable name for the level (trace/debug/info/warn/error).
func (l Level) String() string {
	switch slog.Level(l) {
	case LevelTrace:
		return "trace"
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	case slog.LevelError:
		return "error"
	default:
		return slog.Level(l).String()
	}
}

// ResolveOptions carries the inputs needed to compute the final log level
// when the CLI boots. All fields are optional; zero values fall back to
// environment variables, then to "warn" (which keeps stderr silent for the
// default command path).
type ResolveOptions struct {
	// DebugFlag mirrors --debug. Equivalent to verbosity=1.
	DebugFlag bool
	// Verbosity comes from -v / -vv (count flag). 1 → debug, 2+ → trace.
	Verbosity int
	// Env lets tests inject a fake environment lookup. Nil means os.Getenv.
	Env func(string) string
}

// ResolveLevel returns the active log level given flag overrides and the
// process environment. Precedence is: explicit AONE_LOG_LEVEL > -v/-vv >
// AONE_DEBUG > --debug > default (warn).
func ResolveLevel(opts ResolveOptions) Level {
	getenv := opts.Env
	if getenv == nil {
		getenv = os.Getenv
	}

	// Explicit level wins so power users can pin "warn" or "error" even
	// when something upstream sets AONE_DEBUG.
	if v := strings.TrimSpace(strings.ToLower(getenv("AONE_LOG_LEVEL"))); v != "" {
		if lvl, ok := parseLevel(v); ok {
			return lvl
		}
	}

	if opts.Verbosity >= 2 {
		return Level(LevelTrace)
	}
	if opts.Verbosity == 1 {
		return Level(slog.LevelDebug)
	}

	switch strings.TrimSpace(strings.ToLower(getenv("AONE_DEBUG"))) {
	case "2", "trace":
		return Level(LevelTrace)
	case "1", "true", "yes", "on", "debug":
		return Level(slog.LevelDebug)
	}

	if opts.DebugFlag {
		return Level(slog.LevelDebug)
	}

	return Level(slog.LevelWarn)
}

// parseLevel maps a textual level to its slog/Level value. Used for
// AONE_LOG_LEVEL parsing; unknown strings fall through to the caller.
func parseLevel(s string) (Level, bool) {
	switch s {
	case "trace":
		return Level(LevelTrace), true
	case "debug":
		return Level(slog.LevelDebug), true
	case "info":
		return Level(slog.LevelInfo), true
	case "warn", "warning":
		return Level(slog.LevelWarn), true
	case "error":
		return Level(slog.LevelError), true
	}
	return 0, false
}

// InitOptions configures Init. ResolveOptions is the primary input; tests
// can also override Stderr to capture output, and Now to make timestamps
// deterministic.
type InitOptions struct {
	ResolveOptions
	// Stderr overrides the destination writer. Defaults to os.Stderr; the
	// AONE_LOG_FILE env var, when set, takes precedence over both.
	Stderr io.Writer
}

// state holds the package-wide logger plus the resolved level. We keep
// both so callers can cheaply check "is trace enabled?" without going
// through slog.Handler.Enabled, which would otherwise show up in hot
// paths like loggingTransport.
type state struct {
	mu     sync.RWMutex
	logger *slog.Logger
	level  Level
}

var global = &state{
	logger: slog.New(discardHandler{}),
	level:  Level(slog.LevelError + 1), // higher than anything we emit → off
}

// Init configures the package-wide logger from CLI flags + environment.
// Safe to call multiple times; later calls replace the previous logger.
// Init is a no-op when the resolved level disables all output, keeping the
// default "no flags" path zero-cost on the logging side.
func Init(opts InitOptions) {
	level := ResolveLevel(opts.ResolveOptions)

	dest := opts.Stderr
	if dest == nil {
		dest = os.Stderr
	}
	// AONE_LOG_FILE redirects logs to a file (append mode). When the file
	// can't be opened we silently fall back to stderr so a misconfigured
	// path can never crash the CLI.
	getenv := opts.Env
	if getenv == nil {
		getenv = os.Getenv
	}
	if path := strings.TrimSpace(getenv("AONE_LOG_FILE")); path != "" {
		// 0600 because debug logs may contain partial header dumps even
		// after redaction; better to keep them user-readable only.
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			dest = f
		}
	}

	handlerOpts := &slog.HandlerOptions{Level: slog.Level(level)}
	var handler slog.Handler
	switch strings.TrimSpace(strings.ToLower(getenv("AONE_LOG_FORMAT"))) {
	case "json":
		handler = slog.NewJSONHandler(dest, handlerOpts)
	default:
		handler = slog.NewTextHandler(dest, handlerOpts)
	}

	global.mu.Lock()
	global.logger = slog.New(handler)
	global.level = level
	global.mu.Unlock()
}

// L returns the package-wide logger. Always non-nil; before Init runs (or
// when logging is disabled), it returns a no-op logger so callers never
// need a nil check.
func L() *slog.Logger {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.logger
}

// Enabled reports whether logs at lvl will be emitted. Cheaper than
// constructing log records, useful when the message itself is expensive
// to build (e.g. body dumps).
func Enabled(lvl slog.Level) bool {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return slog.Level(global.level) <= lvl
}

// CurrentLevel returns the resolved level. Mainly for tests; runtime code
// should prefer Enabled.
func CurrentLevel() Level {
	global.mu.RLock()
	defer global.mu.RUnlock()
	return global.level
}

// Trace logs at LevelTrace. Convenience wrapper to keep the level constant
// from leaking into every caller.
func Trace(msg string, args ...any) {
	L().Log(context.Background(), LevelTrace, msg, args...)
}

// Debug logs at slog.LevelDebug.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at slog.LevelInfo.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at slog.LevelWarn.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at slog.LevelError.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// discardHandler is the zero-state handler used before Init runs. It
// answers false to Enabled at every level so call sites short-circuit
// before building log records.
type discardHandler struct{}

// Enabled always returns false so no log record is ever materialized.
func (discardHandler) Enabled(context.Context, slog.Level) bool { return false }

// Handle is unreachable when Enabled returns false; it returns nil for
// safety in case a handler is wrapped without checking Enabled first.
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }

// WithAttrs returns the same handler — attributes are meaningless when
// nothing is ever emitted.
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns the same handler for the same reason.
func (h discardHandler) WithGroup(string) slog.Handler { return h }

// ensure interface satisfaction at compile time.
var _ slog.Handler = discardHandler{}
