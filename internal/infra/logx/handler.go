package logx

import (
	"context"
	"log/slog"
)

// multiHandler 将一条日志扇出到多个 handler。
type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) slog.Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, handler := range handlers {
		if handler != nil {
			filtered = append(filtered, handler)
		}
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &multiHandler{handlers: filtered}
}

// Enabled 仅当任一子 handler 可用时返回 true。
func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle 将记录扇出到所有 handler。
func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// WithAttrs 为所有子 handler 追加字段。
func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		cloned = append(cloned, handler.WithAttrs(attrs))
	}
	return newMultiHandler(cloned...)
}

// WithGroup 为所有子 handler 追加分组。
func (h *multiHandler) WithGroup(name string) slog.Handler {
	cloned := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		cloned = append(cloned, handler.WithGroup(name))
	}
	return newMultiHandler(cloned...)
}
