// INPUT: Agent CLI 输出参数、命令结果与错误。
// OUTPUT: 稳定 JSON envelope、stderr 错误与进程退出码。
// POS: Agent CLI 私有进程外壳；不依赖宿主配置、AppServices 或数据库。
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

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

var currentOutputOptions outputOptions

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

func requestedJSON(arguments []string) bool {
	for _, argument := range arguments {
		switch strings.TrimSpace(argument) {
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

func applyOutputOptions(options outputOptions) error {
	if options.json && options.pretty {
		return usageErrorf("--json 与 --pretty 不能同时使用")
	}
	currentOutputOptions = options
	output := io.Writer(io.Discard)
	if options.verbose {
		output = os.Stderr
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(output, nil)))
	return nil
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

func usageErrorf(format string, arguments ...any) error {
	return usageError(fmt.Errorf(format, arguments...))
}

func exitCode(err error) int {
	if err == nil {
		return exitCodeSuccess
	}
	var current *cliError
	if errors.As(err, &current) && current.kind == cliErrorKindUsage {
		return exitCodeUsage
	}
	return exitCodeExecution
}

func writeCommandError(writer io.Writer, err error, jsonMode bool) {
	if err == nil {
		return
	}
	if jsonMode {
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"success": false,
			"error": map[string]any{
				"kind":    errorKind(err),
				"message": err.Error(),
			},
		})
		return
	}
	_, _ = fmt.Fprintf(writer, "错误: %s\n", err.Error())
	if exitCode(err) == exitCodeUsage {
		_, _ = fmt.Fprintln(writer, "提示: 运行 --help 查看正确用法。")
	}
}

func errorKind(err error) string {
	if exitCode(err) == exitCodeUsage {
		return cliErrorKindUsage
	}
	return cliErrorKindExecution
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("存在额外 JSON 值")
		}
		return err
	}
	return nil
}
