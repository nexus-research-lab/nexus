// =====================================================
// @File   ：output.go
// @Date   ：2026-04-23 10:05
// @Author ：leemysw
// 2026-04-23 10:05   Create
// =====================================================

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"

	"github.com/spf13/cobra"
)

const (
	exitCodeSuccess       = 0
	exitCodeExecution     = 1
	exitCodeUsage         = 64
	cliErrorKindUsage     = "usage"
	cliErrorKindExecution = "execution"
)

type outputOptions struct {
	json    bool
	pretty  bool
	verbose bool
}

type cliError struct {
	kind string
	err  error
}

var currentOutputOptions = outputOptions{}

func (e *cliError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *cliError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func RequestedJSON(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--json", "--json=true":
			return true
		}
	}
	return false
}

func configureRootOutput(root *cobra.Command) *outputOptions {
	options := &outputOptions{}
	root.PersistentFlags().BoolVar(&options.json, "json", false, "以单行 JSON 输出结果，适合 Agent 与脚本消费")
	root.PersistentFlags().BoolVar(&options.pretty, "pretty", false, "以格式化 JSON 输出结果，适合人工阅读")
	root.PersistentFlags().BoolVar(&options.verbose, "verbose", false, "将诊断日志输出到 stderr")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})
	return options
}

func applyOutputOptions(cfg config.Config, services *cliServiceProvider, options outputOptions) error {
	if options.json && options.pretty {
		return usageErrorf("--json 与 --pretty 不能同时使用")
	}
	currentOutputOptions = options

	logger := newCLILogger(cfg, options.verbose)
	slog.SetDefault(logger)
	if services != nil {
		services.SetLogger(logger)
	}
	return nil
}

func newCLILogger(cfg config.Config, verbose bool) *slog.Logger {
	output := io.Discard
	if verbose {
		output = os.Stderr
	}
	return logx.New(logx.Options{
		Service: cfg.ProjectName,
		Level:   cfg.LogLevel,
		Format:  cfg.LogFormat,
		Output:  output,
		Stdout:  false,
		File: logx.FileOptions{
			Enabled:     cfg.LogFileEnabled,
			Path:        cfg.LogPath,
			RotateDaily: cfg.LogRotateDaily,
			MaxSizeMB:   cfg.LogMaxSizeMB,
			MaxAgeDays:  cfg.LogMaxAgeDays,
			MaxBackups:  cfg.LogMaxBackups,
			Compress:    cfg.LogCompress,
		},
		NoColor: true,
	})
}

func bindServiceLogger(services *serverapp.AppServices, logger *slog.Logger) {
	if services == nil || logger == nil {
		return
	}
	if services.Title != nil {
		services.Title.SetLogger(logger.With("component", "title"))
	}
	if services.Channels != nil {
		services.Channels.SetLogger(logger.With("component", "channels"))
	}
	if services.DM != nil {
		services.DM.SetLogger(logger.With("component", "dm"))
	}
	if services.Ingress != nil {
		services.Ingress.SetLogger(logger.With("component", "channels.ingress"))
	}
	if services.RoomRealtime != nil {
		services.RoomRealtime.SetLogger(logger.With("component", "room"))
	}
	if services.Automation != nil {
		services.Automation.SetLogger(logger.With("component", "automation"))
	}
}

func emitJSON(payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["success"]; !ok {
		payload["success"] = true
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if !currentOutputOptions.json {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(payload)
}

func usageError(err error) error {
	if err == nil {
		return nil
	}
	var current *cliError
	if errors.As(err, &current) {
		return err
	}
	return &cliError{kind: cliErrorKindUsage, err: err}
}

func usageErrorf(format string, args ...any) error {
	return usageError(fmt.Errorf(format, args...))
}

func ExitCode(err error) int {
	if err == nil {
		return exitCodeSuccess
	}
	var current *cliError
	if errors.As(err, &current) && current.kind == cliErrorKindUsage {
		return exitCodeUsage
	}
	return exitCodeExecution
}

func WriteCommandError(w io.Writer, err error, jsonMode bool) {
	if err == nil {
		return
	}
	if jsonMode {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error": map[string]any{
				"kind":    errorKind(err),
				"message": err.Error(),
			},
		})
		return
	}
	_, _ = fmt.Fprintf(w, "错误: %s\n", err.Error())
	if ExitCode(err) == exitCodeUsage {
		_, _ = fmt.Fprintln(w, "提示: 运行 --help 查看正确用法。")
	}
}

func errorKind(err error) string {
	if ExitCode(err) == exitCodeUsage {
		return cliErrorKindUsage
	}
	return cliErrorKindExecution
}

func exactArgs(expected int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != expected {
			return usageError(cobra.ExactArgs(expected)(cmd, args))
		}
		return nil
	}
}
