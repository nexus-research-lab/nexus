package runtimeidentity

import (
	"errors"
	"strings"
)

const (
	scriptRuntimeKind = "script"
	maxScriptBytes    = 128 * 1024
)

func validateScriptArguments(args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.New("run-script 只接受一段脚本")
	}
	script := args[0]
	if strings.TrimSpace(script) == "" {
		return "", errors.New("run-script 脚本不能为空")
	}
	if strings.ContainsRune(script, 0) {
		return "", errors.New("run-script 脚本包含 NUL")
	}
	if len(script) > maxScriptBytes {
		return "", errors.New("run-script 脚本超过 128 KiB")
	}
	return script, nil
}
