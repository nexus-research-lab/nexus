// INPUT: 产品已解析的强类型 SandboxSettings/CWD/Resources 与可信端点清单。
// OUTPUT: SDK RuntimeConfig 的窄 JSON 投影；未知/弱化配置拒绝，不接收任务RawMessage。
// POS: 跨仓线格式适配而非SDK依赖，完整命令在Bridge Reserve时另外绑定。
package runtime

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type WindowsSandboxNetworkEndpoint struct {
	Scheme    string   `json:"scheme"`
	Host      string   `json:"host"`
	Port      uint16   `json:"port"`
	Addresses []string `json:"addresses"`
}

func windowsSandboxBool(value *bool) bool { return value != nil && *value }

// projectWindowsSandboxConfig 保留presence与显式deny；不把macOS设置直接当Windows系统权限。
func projectWindowsSandboxConfig(options bridge.Options) (json.RawMessage, error) {
	s := options.Sandbox
	if normalizedManagedRuntimeKind(options.Runtime.Kind) != bridge.RuntimeNXS || s == nil || !s.RequireSandbox || !windowsSandboxBool(s.Enabled) || !windowsSandboxBool(s.FailIfUnavailable) || s.Resources == nil || !filepath.IsAbs(options.CWD) {
		return nil, errors.New("Windows supervision requires explicit restricted nxs resource policy")
	}
	if err := s.Resources.Validate(); err != nil {
		return nil, err
	}
	if len(s.Extra) != 0 || windowsSandboxBool(s.AllowUnsandboxedCommands) || windowsSandboxBool(s.EnableWeakerNestedSandbox) || windowsSandboxBool(s.EnableWeakerNetworkIsolation) || windowsSandboxBool(s.AllowAppleEvents) || windowsSandboxBool(s.AllowPty) || len(s.ExcludedCommands) != 0 || s.Seccomp != nil || len(s.IgnoreViolations) != 0 || s.RequireClaudeNativeSandbox || s.RequireClaudeRestricted {
		return nil, errors.New("Windows policy projection refuses unsupported escape or opaque settings")
	}
	if s.EnabledPlatforms != nil && !slices.Contains(s.EnabledPlatforms, "windows") {
		return nil, errors.New("Windows is absent from explicit enabled sandbox platforms")
	}
	config := map[string]any{"RequireSandbox": true, "Enabled": true, "FailIfUnavailable": true, "Platform": "windows", "PolicyRoot": options.CWD, "Resources": s.Resources, "AllowUnsandboxedCommands": false, "EnabledPlatforms": []string{"windows"}, "EnabledPlatformsConfigured": true, "AutoAllowBashIfSandboxed": windowsSandboxBool(s.AutoAllowBashIfSandboxed)}
	if s.Filesystem != nil {
		f := s.Filesystem
		config["AllowRead"], config["AllowWrite"], config["DenyRead"], config["DenyWrite"] = f.AllowRead, f.AllowWrite, f.DenyRead, f.DenyWrite
		config["AllowManagedReadPathsOnly"], config["AllowGitConfig"] = windowsSandboxBool(f.AllowManagedReadPathsOnly), windowsSandboxBool(f.AllowGitConfig)
	}
	if s.Ripgrep != nil {
		config["Ripgrep"] = struct {
			Command string
			Args    []string
			Argv0   string
		}{s.Ripgrep.Command, s.Ripgrep.Args, s.Ripgrep.Argv0}
	}
	if s.MandatoryDenySearchDepth != nil {
		config["MandatoryDenySearchDepth"] = *s.MandatoryDenySearchDepth
	}
	network := map[string]any{"AllowedDomains": []string{}, "AllowedDomainsConfigured": true}
	if s.Network != nil {
		n := s.Network
		if len(n.AllowUnixSockets) != 0 || windowsSandboxBool(n.AllowAllUnixSockets) || windowsSandboxBool(n.AllowLocalBinding) || len(n.AllowMachLookup) != 0 || n.HTTPProxyPort != nil || n.SOCKSProxyPort != nil || n.MITMProxy != nil {
			return nil, errors.New("Windows network projection refuses unsupported local/proxy policy")
		}
		network["AllowedDomains"], network["DeniedDomains"], network["AllowManagedDomainsOnly"] = n.AllowedDomains, n.DeniedDomains, windowsSandboxBool(n.AllowManagedDomainsOnly)
	}
	config["Network"] = network
	return json.Marshal(config)
}
