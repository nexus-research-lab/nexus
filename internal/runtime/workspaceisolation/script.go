package workspaceisolation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const scriptLauncherWaitDelay = 5 * time.Second

// 与 launcher 的 argv 校验保持同一上限，避免把超大脚本交给 exec。
const maxScriptBytes = 128 * 1024

// ScriptInput 描述一段必须以 owner OS 身份执行的自动化脚本。
type ScriptInput struct {
	OwnerUserID string
	CWD         string
	Script      string
	Environment map[string]string
}

// RunScript 通过 root-owned launcher 执行自动化脚本。
//
// launcher 会重新确认 owner/cwd、以目录 fd 进入 workspace，再应用
// UID/GID、项目组与 Landlock。这里不提供 audit/off 回退，避免 server 在配置
// 拼写或漏配时静默退回宿主身份执行。
func RunScript(
	ctx context.Context,
	config Config,
	input ScriptInput,
	stdout io.Writer,
	stderr io.Writer,
) error {
	mode, err := NormalizeMode(string(config.Mode))
	if err != nil {
		return err
	}
	if mode != ModeEnforce {
		return errors.New("server script automation requires runtime isolation enforce mode")
	}
	if runtime.GOOS != "linux" {
		return errors.New("server script automation isolation is only available on Linux")
	}

	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.CWD = filepath.Clean(strings.TrimSpace(input.CWD))
	if input.OwnerUserID == "" || input.CWD == "" || input.CWD == "." {
		return errors.New("script isolation 缺少 owner 或 workspace")
	}
	if strings.TrimSpace(input.Script) == "" {
		return errors.New("script automation instruction is empty")
	}
	if strings.ContainsRune(input.Script, 0) {
		return errors.New("script automation instruction contains NUL")
	}
	if len(input.Script) > maxScriptBytes {
		return errors.New("script automation instruction exceeds 128 KiB")
	}

	environmentNames := make([]string, 0, len(input.Environment))
	for name := range input.Environment {
		environmentNames = append(environmentNames, name)
	}
	slices.Sort(environmentNames)
	policy, err := prepareLauncherPolicy(ctx, config.LauncherPath, Input{
		OwnerUserID:      input.OwnerUserID,
		RuntimeKind:      ScriptRuntimeKind,
		CWD:              input.CWD,
		EnvironmentNames: environmentNames,
	})
	if err != nil {
		return err
	}

	launcherPath := filepath.Clean(strings.TrimSpace(config.LauncherPath))
	command := exec.CommandContext(ctx, launcherPath, "run-script", input.Script)
	command.Env = scriptLauncherEnvironment(policy.Ticket, input.Environment)
	command.Stdout = stdout
	command.Stderr = stderr
	// CommandContext 默认直接 SIGKILL setuid launcher，可能来不及回收 shell
	// 子进程。先发 interrupt，让 launcher 以 root supervisor 身份清理进程组。
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		if signalErr := command.Process.Signal(os.Interrupt); signalErr != nil {
			return fmt.Errorf("interrupt script launcher: %w", signalErr)
		}
		return nil
	}
	command.WaitDelay = scriptLauncherWaitDelay
	if err = command.Run(); err != nil {
		return fmt.Errorf("run isolated automation script: %w", err)
	}
	return nil
}

func scriptLauncherEnvironment(ticket string, environment map[string]string) []string {
	values := map[string]string{
		LauncherTicketEnvName: strings.TrimSpace(ticket),
		LauncherModeEnvName:   string(ModeEnforce),
		"LANG":                "C.UTF-8",
	}
	for name, value := range environment {
		name = strings.TrimSpace(name)
		if name == "" || name == LauncherTicketEnvName || name == LauncherModeEnvName {
			continue
		}
		values[name] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+values[name])
	}
	return result
}
