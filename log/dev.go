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
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"sync"
)

type devLevelName string

const (
	// devLevelDebugName is the compact debug label used in the local dev log.
	devLevelDebugName devLevelName = "DBG"
	// devLevelInfoName is the compact info label used in the local dev log.
	devLevelInfoName devLevelName = "INF"
	// devLevelWarnName is the compact warning label used in the local dev log.
	devLevelWarnName devLevelName = "WRN"
	// devLevelErrorName is the compact error label used in the local dev log.
	devLevelErrorName devLevelName = "ERR"
)

type ansiColorCode string

const (
	// ansiYellow is used for debug logs so local diagnostic output is visible
	// without competing with warnings and errors.
	ansiYellow ansiColorCode = "33"
	// ansiCyan is used for ordinary info logs.
	ansiCyan ansiColorCode = "36"
	// ansiMagenta is used for warnings to distinguish them from errors.
	ansiMagenta ansiColorCode = "35"
	// ansiRed is reserved for errors.
	ansiRed ansiColorCode = "31"
)

type devLevelStyle struct {
	name  devLevelName
	color ansiColorCode
}

type devHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
	color bool
	mu    *sync.Mutex
}

// newDevHandler intentionally avoids third-party dependencies. It gives local
// development a tint-style compact format while production can use slog's text
// or JSON handlers.
func newDevHandler(w io.Writer, level slog.Level) slog.Handler {
	return &devHandler{
		w:     w,
		level: level,
		color: isTerminal(w),
		mu:    &sync.Mutex{},
	}
}

func (h *devHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *devHandler) Handle(_ context.Context, record slog.Record) error {
	var b strings.Builder
	// Keep timestamps short and millisecond-precision so boot/poll loops are
	// easy to scan when running Shoelaces locally.
	b.WriteString(record.Time.Format("[15:04:05.000]"))
	b.WriteByte(' ')
	b.WriteString(h.levelName(record.Level))
	if record.Message != "" {
		b.WriteByte(' ')
		b.WriteString(record.Message)
	}

	writeAttr := func(attr slog.Attr) {
		// Resolve LogValuer values before formatting so dynamic attributes behave
		// consistently with the standard slog handlers.
		attr.Value = attr.Value.Resolve()
		if attr.Equal(slog.Attr{}) {
			return
		}
		key := attr.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		b.WriteByte(' ')
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatValue(attr.Value.Any()))
	}

	for _, attr := range h.attrs {
		writeAttr(attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		writeAttr(attr)
		return true
	})
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

func (h *devHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Copy-on-write keeps derived loggers independent while sharing the same
	// writer lock, matching slog handler expectations.
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

func (h *devHandler) WithGroup(name string) slog.Handler {
	next := *h
	if next.group == "" {
		next.group = name
	} else {
		next.group += "." + name
	}
	return &next
}

func (h *devHandler) levelName(level slog.Level) string {
	style := devStyleForLevel(level)
	if !h.color {
		return string(style.name)
	}
	return "\x1b[" + string(style.color) + "m" + string(style.name) + "\x1b[0m"
}

func devStyleForLevel(level slog.Level) devLevelStyle {
	switch {
	case level <= slog.LevelDebug:
		return devLevelStyle{name: devLevelDebugName, color: ansiYellow}
	case level >= slog.LevelError:
		return devLevelStyle{name: devLevelErrorName, color: ansiRed}
	case level >= slog.LevelWarn:
		return devLevelStyle{name: devLevelWarnName, color: ansiMagenta}
	default:
		return devLevelStyle{name: devLevelInfoName, color: ansiCyan}
	}
}

func formatValue(v any) string {
	// Quote only values that would make key=value output ambiguous. This keeps
	// common IDs and paths compact while preserving spaces and error text.
	if isNilValue(v) {
		return "<nil>"
	}
	switch value := v.(type) {
	case string:
		if value == "" || strings.ContainsAny(value, " \t\n\r\"=") {
			return fmt.Sprintf("%q", value)
		}
		return value
	case error:
		return fmt.Sprintf("%q", value.Error())
	case fmt.Stringer:
		return fmt.Sprintf("%q", value.String())
	default:
		return fmt.Sprint(value)
	}
}

func isNilValue(v any) bool {
	if v == nil {
		return true
	}
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
