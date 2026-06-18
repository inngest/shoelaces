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
	"strings"
	"sync"
)

type devHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
	color bool
	mu    *sync.Mutex
}

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
	b.WriteString(record.Time.Format("[15:04:05.000]"))
	b.WriteByte(' ')
	b.WriteString(h.levelName(record.Level))
	if record.Message != "" {
		b.WriteByte(' ')
		b.WriteString(record.Message)
	}

	writeAttr := func(attr slog.Attr) {
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
	name := "INF"
	color := "36"
	switch {
	case level <= slog.LevelDebug:
		name = "DBG"
		color = "33"
	case level >= slog.LevelError:
		name = "ERR"
		color = "31"
	case level >= slog.LevelWarn:
		name = "WRN"
		color = "35"
	}
	if !h.color {
		return name
	}
	return "\x1b[" + color + "m" + name + "\x1b[0m"
}

func formatValue(v any) string {
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

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
