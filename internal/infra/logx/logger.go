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
	NoColor bool
}

// New 创建结构化日志实例。
func New(options Options) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{
		Level: parseLevel(options.Level),
	}
	handler := buildHandler(options, handlerOptions)

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

func buildHandler(options Options, handlerOptions *slog.HandlerOptions) slog.Handler {
	handlers := make([]slog.Handler, 0, 3)
	if options.Output != nil {
		handlers = append(handlers, newHandlerForWriter(options.Output, options.Format, handlerOptions, false))
	}
	if options.Stdout {
		handlers = append(handlers, newHandlerForWriter(os.Stdout, options.Format, handlerOptions, shouldColorize(os.Stdout, options.NoColor)))
	}
	if fileWriter, err := newRollingFileWriter(options.File); err == nil && fileWriter != nil {
		handlers = append(handlers, newHandlerForWriter(fileWriter, options.Format, handlerOptions, false))
	} else if err != nil {
		_, _ = os.Stderr.WriteString("init log file writer failed: " + err.Error() + "\n")
	}

	switch len(handlers) {
	case 0:
		return slog.NewTextHandler(io.Discard, handlerOptions)
	case 1:
		return handlers[0]
	default:
		return newMultiHandler(handlers...)
	}
}

func newHandlerForWriter(
	writer io.Writer,
	format string,
	handlerOptions *slog.HandlerOptions,
	colorize bool,
) slog.Handler {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		return slog.NewJSONHandler(writer, handlerOptions)
	default:
		return newPrettyHandler(writer, handlerOptions, colorize)
	}
}

func shouldColorize(writer io.Writer, noColor bool) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0 && !noColor
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
