// INPUT: runtime bridge options、部署隔离配置与宿主可信 owner/workspace/主体快照。
// OUTPUT: 普通 Agent 全模式 raw nexusctl deny；主智能体 control-plane Hook 或普通 Agent enforce launcher/options。
// POS: nxs/Claude runtime 隔离与 owner-scoped 主智能体控制面的统一装配入口。
package workspaceisolation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

// Apply 为 nxs/Claude 注入同一个 PreToolUse policy；普通 Agent 在 enforce
// 模式把 bridge 进程入口切到 root-owned launcher，使 UID/GID、ACL 与
// Landlock 不依赖 runtime。主智能体保留 owner-scoped 宿主控制面身份。
func Apply(
	ctx context.Context,
	options agentclient.Options,
	config Config,
	input Input,
) (agentclient.Options, error) {
	mode, err := NormalizeMode(string(config.Mode))
	if err != nil {
		return agentclient.Options{}, err
	}
	if mode == ModeOff {
		if !input.IsMainAgent {
			options = withRawNexusctlDenyHook(options)
		}
		return options, nil
	}
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.RuntimeKind = strings.TrimSpace(input.RuntimeKind)
	input.CWD = strings.TrimSpace(input.CWD)
	if input.OwnerUserID == "" || input.RuntimeKind == "" || input.CWD == "" {
		return agentclient.Options{}, errors.New("runtime isolation 缺少 owner、runtime 或 workspace")
	}

	var policy Policy
	if mode == ModeEnforce {
		if input.IsMainAgent {
			if runtime.GOOS != "linux" {
				return agentclient.Options{}, errors.New("runtime isolation enforce 目前只支持 Linux")
			}
			if err = validateEnforceOptions(options); err != nil {
				return agentclient.Options{}, err
			}
			policy, err = buildAuditPolicy(input)
			if err != nil {
				return agentclient.Options{}, err
			}
			return withWorkspacePolicyHook(options, mode, policy), nil
		}
		if err = validateEnforceOptions(options); err != nil {
			return agentclient.Options{}, err
		}
		if runtime.GOOS != "linux" {
			return agentclient.Options{}, errors.New("runtime isolation enforce 目前只支持 Linux")
		}
		input.EnvironmentNames = sortedEnvironmentNames(options.Env)
		policy, err = prepareLauncherPolicy(ctx, config.LauncherPath, input)
		if err != nil {
			return agentclient.Options{}, err
		}
		options.CLIPath = filepath.Clean(strings.TrimSpace(config.LauncherPath))
		if options.Env == nil {
			options.Env = map[string]string{}
		}
		options.Env[LauncherTicketEnvName] = policy.Ticket
		options.Env[LauncherModeEnvName] = string(mode)
	} else {
		policy, err = buildAuditPolicy(input)
		if err != nil {
			return agentclient.Options{}, err
		}
	}
	return withWorkspacePolicyHook(options, mode, policy), nil
}

func validateEnforceOptions(options agentclient.Options) error {
	if options.Transport != nil || options.DirectConnect != nil ||
		strings.TrimSpace(options.Executable) != "" ||
		strings.TrimSpace(options.PathToExecutable) != "" ||
		len(options.ExecutableArgs) != 0 ||
		len(options.ExtraArgs) != 0 ||
		len(options.ExtraBoolArgs) != 0 ||
		strings.TrimSpace(options.User) != "" {
		return errors.New("enforce runtime 只能使用受控的本地 launcher process transport")
	}
	return nil
}

func prepareLauncherPolicy(
	ctx context.Context,
	launcherPath string,
	input Input,
) (Policy, error) {
	launcherPath = filepath.Clean(strings.TrimSpace(launcherPath))
	if launcherPath == "." || !filepath.IsAbs(launcherPath) {
		return Policy{}, errors.New("NEXUS_RUNTIME_LAUNCHER_PATH 必须是绝对路径")
	}
	info, err := os.Stat(launcherPath)
	if err != nil {
		return Policy{}, fmt.Errorf("读取 runtime launcher: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return Policy{}, fmt.Errorf("runtime launcher 不可执行: %s", launcherPath)
	}
	if err = validateLauncherBinary(launcherPath); err != nil {
		return Policy{}, err
	}

	arguments := []string{
		"prepare",
		"--owner", input.OwnerUserID,
		"--runtime", input.RuntimeKind,
		"--cwd", input.CWD,
	}
	for _, root := range input.ReadRoots {
		if trimmed := strings.TrimSpace(root); trimmed != "" {
			arguments = append(arguments, "--read-root", trimmed)
		}
	}
	for _, name := range input.EnvironmentNames {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			arguments = append(arguments, "--env", trimmed)
		}
	}
	command := exec.CommandContext(ctx, launcherPath, arguments...)
	command.Env = []string{
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"LANG=C.UTF-8",
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err = command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return Policy{}, fmt.Errorf("准备 runtime isolation policy 失败: %s", detail)
	}
	var policy Policy
	if err = json.Unmarshal(stdout.Bytes(), &policy); err != nil {
		return Policy{}, fmt.Errorf("解析 runtime launcher policy: %w", err)
	}
	if err = policy.validate(input, true); err != nil {
		return Policy{}, fmt.Errorf("校验 runtime launcher policy: %w", err)
	}
	return policy, nil
}

func sortedEnvironmentNames(environment map[string]string) []string {
	names := make([]string, 0, len(environment))
	for name := range environment {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func buildAuditPolicy(input Input) (Policy, error) {
	if appfs.UserPathSegment(input.OwnerUserID) != strings.TrimSpace(input.OwnerUserID) {
		return Policy{}, errors.New("owner user id 不能安全映射为 workspace 路径")
	}
	workspaceRoot := appfs.UserWorkspaceRoot(input.OwnerUserID)
	// audit 需要兼容部署级自定义 workspace：它只记录早期策略命中，
	// 不把这条路径当作 enforce 的 OS 授权事实源。
	readRoots := append([]string{workspaceRoot, input.CWD}, input.ReadRoots...)
	writeRoots := append([]string{workspaceRoot, input.CWD}, input.WriteRoots...)
	if sharedTempRoot := appfs.RuntimeSharedTempRoot(); sharedTempRoot != "" {
		readRoots = append(readRoots, sharedTempRoot)
		writeRoots = append(writeRoots, sharedTempRoot)
	}
	cwd := input.CWD
	var err error
	normalizedRead, err := normalizePolicyRoots(readRoots)
	if err != nil {
		return Policy{}, err
	}
	normalizedWrite, err := normalizePolicyRoots(writeRoots)
	if err != nil {
		return Policy{}, err
	}
	policy := Policy{
		OwnerUserID: input.OwnerUserID,
		IsMainAgent: input.IsMainAgent,
		RuntimeKind: input.RuntimeKind,
		CWD:         cwd,
		ReadRoots:   normalizedRead,
		WriteRoots:  normalizedWrite,
	}
	if err = policy.validate(input, false); err != nil {
		return Policy{}, err
	}
	return policy, nil
}
