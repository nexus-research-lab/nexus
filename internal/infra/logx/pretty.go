package logx

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
)

type prettyHandler struct {
	writer   io.Writer
	level    slog.Leveler
	attrs    []slog.Attr
	groups   []string
	mutex    *sync.Mutex
	colorize bool
}

func newPrettyHandler(writer io.Writer, options *slog.HandlerOptions, colorize bool) slog.Handler {
	var level slog.Leveler = slog.LevelInfo
	if options != nil && options.Level != nil {
		level = options.Level
	}
	return &prettyHandler{
		writer:   writer,
		level:    level,
		mutex:    &sync.Mutex{},
		colorize: colorize,
	}
}

func (h *prettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := *h
	cloned.attrs = slices.Concat(h.attrs, attrs)
	return &cloned
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if strings.TrimSpace(name) == "" {
		return h
	}
	cloned := *h
	cloned.groups = slices.Concat(h.groups, []string{name})
	return &cloned
}

func (h *prettyHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]field, 0, record.NumAttrs()+len(h.attrs))
	fields = appendAttrs(fields, h.attrs, h.groups)
	record.Attrs(func(attr slog.Attr) bool {
		fields = appendAttr(fields, attr, h.groups)
		return true
	})

	scope, fields := pickScope(fields)
	access, fields := pickAccess(fields)
	requestID, fields := pickRequestID(fields)
	sdkDebug, fields := pickSDKDebug(fields)

	line := h.format(record.Time, record.Level, scope, record.Message, access, requestID, sdkDebug, fields)

	h.mutex.Lock()
	defer h.mutex.Unlock()
	_, err := io.WriteString(h.writer, line)
	return err
}
