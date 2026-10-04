package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// line writes one log record as
//
//	2026-10-04T13:34:56.456 INFO listen.go:343:begin() - recording started kind=meeting
//
// The time, level and source come first; the attributes follow as slog's text
// handler writes them, quoting and groups included.
type line struct {
	attrs slog.Handler // a text handler that writes only the attributes, into buf
	buf   *bytes.Buffer
	out   io.Writer
	mu    *sync.Mutex
}

func newLine(out io.Writer, level slog.Leveler) slog.Handler {
	buf := new(bytes.Buffer)
	return line{out: out, buf: buf, mu: new(sync.Mutex), attrs: slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.MessageKey) {
				return slog.Attr{}
			}
			return a
		},
	})}
}

func (h line) Enabled(ctx context.Context, level slog.Level) bool {
	return h.attrs.Enabled(ctx, level)
}

func (h line) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.attrs = h.attrs.WithAttrs(attrs)
	return h
}

func (h line) WithGroup(name string) slog.Handler {
	h.attrs = h.attrs.WithGroup(name)
	return h
}

func (h line) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf.Reset()
	if err := h.attrs.Handle(ctx, r); err != nil {
		return err
	}
	// github.com/…/internal/listen.(*Recorder).begin → begin
	f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
	name := f.Function[strings.LastIndexByte(f.Function, '/')+1:]
	name = name[strings.IndexByte(name, '.')+1:]
	if i := strings.Index(name, ")."); i >= 0 {
		name = name[i+2:]
	}
	out := fmt.Appendf(nil, "%s %s %s:%d:%s() - %s", r.Time.Format("2006-01-02T15:04:05.000"),
		r.Level, filepath.Base(f.File), f.Line, name, r.Message)
	if attrs := bytes.TrimSpace(h.buf.Bytes()); len(attrs) > 0 {
		out = append(append(out, ' '), attrs...)
	}
	_, err := h.out.Write(append(out, '\n'))
	return err
}
