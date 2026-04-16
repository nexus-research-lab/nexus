// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：logger.go
// @Date   ：2026/04/11 20:21:00
// @Author ：leemysw
// 2026/04/11 20:21:00   Create
// =====================================================

package logx

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Options 描述日志构造参数。
type Options struct {
	Service string
	Level   string
	Format  string
	Output  io.Writer
	Stdout  bool
	File    FileOptions
}

// New 创建结构化日志实例。
func New(options Options) *slog.Logger {
	output := resolveOutput(options)

	handlerOptions := &slog.HandlerOptions{
		Level: parseLevel(options.Level),
	}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(options.Format)) {
	case "json":
		handler = slog.NewJSONHandler(output, handlerOptions)
	default:
		handler = slog.NewTextHandler(output, handlerOptions)
	}

	logger := slog.New(handler)
	if service := strings.TrimSpace(options.Service); service != "" {
		logger = logger.With("service", service)
	}
	return logger
}

// NewDiscardLogger 返回一个丢弃输出的 logger，适合测试场景。
func NewDiscardLogger() *slog.Logger {
	return New(Options{Output: io.Discard})
}

func resolveOutput(options Options) io.Writer {
	outputs := make([]io.Writer, 0, 2)
	if options.Output != nil {
		outputs = append(outputs, options.Output)
	}
	if options.Stdout {
		outputs = append(outputs, os.Stdout)
	}
	if fileWriter, err := newRollingFileWriter(options.File); err == nil && fileWriter != nil {
		outputs = append(outputs, fileWriter)
	} else if err != nil {
		_, _ = os.Stderr.WriteString("init log file writer failed: " + err.Error() + "\n")
	}

	switch len(outputs) {
	case 0:
		return io.Discard
	case 1:
		return outputs[0]
	default:
		return io.MultiWriter(outputs...)
	}
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
