// Copyright 2026 Inngest Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Handler string

const (
	// HandlerDev is optimized for local terminal use: compact levels, readable
	// attributes, and color when stdout/stderr is a TTY.
	HandlerDev  Handler = "dev"
	HandlerJSON Handler = "json"
	HandlerText Handler = "text"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Logger is the project logging surface. It intentionally mirrors the common
// slog methods without making callers import log/slog directly.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// Option mutates logger construction without exposing slog-specific config to
// callers. Keep new runtime knobs here instead of widening call-site imports.
type Option func(*options)

type options struct {
	level      slog.Level
	levelSet   bool
	handler    Handler
	handlerSet bool
}

// WithLevel sets the minimum level for the logger.
func WithLevel(level Level) Option {
	return func(o *options) {
		o.level = slogLevel(level)
		o.levelSet = true
	}
}

// WithLevelString sets the minimum level from a string such as "debug".
func WithLevelString(level string) Option {
	return func(o *options) {
		if level == "" {
			return
		}
		o.level = slogLevel(ParseLevel(level))
		o.levelSet = true
	}
}

// WithHandler sets the slog handler used by the logger.
func WithHandler(handler Handler) Option {
	return func(o *options) {
		o.handler = normalizeHandler(string(handler))
		o.handlerSet = true
	}
}

// WithHandlerString sets the slog handler from a string such as "json".
func WithHandlerString(handler string) Option {
	return func(o *options) {
		if handler == "" {
			return
		}
		o.handler = normalizeHandler(handler)
		o.handlerSet = true
	}
}

// MakeLogger returns a logger configured with Shoelaces' output defaults.
func MakeLogger(w io.Writer, opts ...Option) Logger {
	// Environment variables match Inngest's logger conventions and let operators
	// change output in containers without editing Shoelaces config files.
	o := options{
		level:   slogLevel(ParseLevel(os.Getenv("LOG_LEVEL"))),
		handler: normalizeHandler(os.Getenv("LOG_HANDLER")),
	}
	for _, apply := range opts {
		apply(&o)
	}
	// Empty env vars should not be treated as explicit overrides. This preserves
	// the normal default of info-level dev logs while still allowing options to
	// intentionally set either field.
	if !o.levelSet && os.Getenv("LOG_LEVEL") == "" {
		o.level = slogLevel(LevelInfo)
	}
	if !o.handlerSet && os.Getenv("LOG_HANDLER") == "" {
		o.handler = HandlerDev
	}
	if w == nil {
		w = io.Discard
	}

	return logger{slog.New(newHandler(w, o.level, o.handler))}
}

// ParseLevel parses Shoelaces log levels. Unknown values fall back to info.
func ParseLevel(level string) Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return LevelDebug
	case "info", "":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

type logger struct {
	*slog.Logger
}

// With keeps the return type on this package's interface, so callers can pass
// scoped loggers around without depending on slog's concrete logger type.
func (l logger) With(args ...any) Logger {
	return logger{l.Logger.With(args...)}
}

func slogLevel(level Level) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func newHandler(w io.Writer, level slog.Level, handler Handler) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	switch handler {
	case HandlerJSON:
		return slog.NewJSONHandler(w, opts)
	case HandlerText:
		return slog.NewTextHandler(w, opts)
	default:
		return newDevHandler(w, level)
	}
}

func normalizeHandler(handler string) Handler {
	switch strings.ToLower(strings.TrimSpace(handler)) {
	case "json":
		return HandlerJSON
	case "txt", "text":
		return HandlerText
	default:
		return HandlerDev
	}
}
