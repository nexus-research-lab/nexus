// INPUT: 安装包内 nxs 的显式绝对路径与当前 sidecar 的产品配置装配。
// OUTPUT: 真实能力协商及清理成功的二进制摘要、Bridge 版本和能力证据。
// POS: 分发前的版本配套检查；所有状态位于新建临时目录，不接触用户数据。
package runtimecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

// Report 只记录被检查文件及已完成握手的事实，不声明发布验收。
type Report struct {
	Version       int      `json:"version"`
	Platform      string   `json:"platform"`
	Architecture  string   `json:"architecture"`
	RuntimeSHA256 string   `json:"runtime_sha256"`
	Bridge        string   `json:"bridge"`
	Profiles      []string `json:"profiles"`
}

// Check 用实际 sidecar 的依赖和产品策略检查随包内核，缺失能力直接终止打包。
func Check(ctx context.Context, binary string) (Report, error) {
	if runtime.GOOS != "darwin" {
		return Report{}, fmt.Errorf("native macOS runtime compatibility check required")
	}
	if !filepath.IsAbs(binary) {
		return Report{}, fmt.Errorf("nxs path must be absolute")
	}
	file, err := os.Open(binary)
	if err != nil {
		return Report{}, err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	closeErr := file.Close()
	if err != nil {
		return Report{}, err
	}
	if closeErr != nil {
		return Report{}, closeErr
	}
	report := Report{Version: 1, Platform: runtime.GOOS, Architecture: runtime.GOARCH, RuntimeSHA256: hex.EncodeToString(hash.Sum(nil))}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range info.Deps {
			if dependency.Path == "github.com/nexus-research-lab/nexus-agent-sdk-bridge" {
				if dependency.Replace != nil {
					return Report{}, fmt.Errorf("release compatibility cannot use a replaced Bridge module")
				}
				report.Bridge = dependency.Version
			}
		}
	}
	root, err := os.MkdirTemp("", "nexus-runtime-compatibility-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(root)
	for _, profile := range []string{"workspace-write", "read-only", "full-access"} {
		if err := checkProfile(ctx, binary, filepath.Join(root, profile), profile); err != nil {
			return Report{}, fmt.Errorf("bundled nxs is incompatible with this App (%s): %w", profile, err)
		}
		report.Profiles = append(report.Profiles, profile)
	}
	return report, nil
}

// checkProfile 复用产品 options builder，独立完成 initialize 和 close，不发送 prompt。
func checkProfile(ctx context.Context, binary, root, profile string) error {
	workspace := filepath.Join(root, "workspace")
	scratch := filepath.Join(root, "scratch")
	for _, dir := range []string{workspace, scratch, filepath.Join(root, "home"), filepath.Join(root, "config")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	input := clientopts.AgentClientOptionsInput{
		WorkspacePath: workspace, RuntimeKind: "nxs", AppMode: "desktop",
		PermissionMode: sdkpermission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true,
		SettingSources:   []string{"user"},
		SandboxResources: &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: scratch},
	}
	if profile == "read-only" {
		input.SandboxResources.WriteScope = agentclient.SandboxWriteScopeReadOnly
	} else if profile == "full-access" {
		input.PermissionMode = sdkpermission.ModeBypassPermissions
		input.SandboxResources = nil
	}
	options, err := clientopts.BuildAgentClientOptions(ctx, nil, input)
	if err != nil {
		return err
	}
	options.CLIPath = binary
	options.Env["HOME"] = filepath.Join(root, "home")
	options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
	options.Env["CLAUDE_CONFIG_DIR"] = filepath.Join(root, "config")
	options.Env["TMPDIR"] = scratch
	options.Skills = agentclient.SkillOptions{Mode: agentclient.SkillModeNone}
	options.MCP.StrictConfig = true
	options.Runtime.InitializeTimeout = 15 * time.Second
	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		return err
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return session.Close(closeCtx)
}
