// INPUT: 宿主准备的 runtime options 与当前 Manager session identity。
// OUTPUT: 不含 prompt、tool schema 明文或密钥的 cache correlation surface。
// POS: runtime 会话状态中的低敏可观测性投影；它不是 provider cache key。
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

const (
	managedExecutionMCPServerName = "nexus_execution"
	runtimeProviderEnvName        = "NEXUS_RUNTIME_PROVIDER"
	cacheSurfaceInspectionTimeout = 100 * time.Millisecond
)

var cacheSurfaceInspectionSlots = make(chan struct{}, 8)

// CacheSurfaceProfile 是一次 runtime 配置的脱敏 cache 相关切面。
// ToolSurfaceFingerprint 只覆盖宿主可证明的 tool policy、MCP 配置以及
// Nexus 内建 SDK server 的 model-visible tools/list；不声称等于 provider cache key。
type CacheSurfaceProfile struct {
	RuntimeKind                 string
	ProviderFingerprint         string
	ModelFingerprint            string
	GoalToolSurfacePresent      bool
	ExecutionToolSurfacePresent bool
	ManagedToolPresenceKnown    bool
	HostToolSurfaceComplete     bool
	ManagedToolSurfaceComplete  bool
	ToolPolicyFingerprint       string
	MCPServersFingerprint       string
	ToolSurfaceFingerprint      string
}

// CacheSurfaceInput 是 runtime 向 usage 投影的低敏 DTO。它避免 usage service
// 反向依赖 Manager 的内部 profile，同时让 DM/Room 共用唯一转换入口。
type CacheSurfaceInput struct {
	RuntimeKind                 string
	ProviderFingerprint         string
	ModelFingerprint            string
	GoalToolSurfacePresent      bool
	ExecutionToolSurfacePresent bool
	ManagedToolPresenceKnown    bool
	HostToolSurfaceComplete     bool
	ManagedToolSurfaceComplete  bool
	ToolPolicyFingerprint       string
	MCPServersFingerprint       string
	ToolSurfaceFingerprint      string
}

// Input 返回不含 prompt、schema 或 tool list 明文的隔离副本。
func (p CacheSurfaceProfile) Input() CacheSurfaceInput {
	return CacheSurfaceInput{
		RuntimeKind:                 p.RuntimeKind,
		ProviderFingerprint:         p.ProviderFingerprint,
		ModelFingerprint:            p.ModelFingerprint,
		GoalToolSurfacePresent:      p.GoalToolSurfacePresent,
		ExecutionToolSurfacePresent: p.ExecutionToolSurfacePresent,
		ManagedToolPresenceKnown:    p.ManagedToolPresenceKnown,
		HostToolSurfaceComplete:     p.HostToolSurfaceComplete,
		ManagedToolSurfaceComplete:  p.ManagedToolSurfaceComplete,
		ToolPolicyFingerprint:       p.ToolPolicyFingerprint,
		MCPServersFingerprint:       p.MCPServersFingerprint,
		ToolSurfaceFingerprint:      p.ToolSurfaceFingerprint,
	}
}

type cacheToolSurfaceManifest struct {
	Version                     int               `json:"version"`
	NativeToolSurface           string            `json:"native_tool_surface"`
	ToolPolicyFingerprint       string            `json:"tool_policy_fingerprint"`
	MCPServersFingerprint       string            `json:"mcp_servers_fingerprint"`
	GoalToolSurfacePresent      bool              `json:"goal_tool_surface_present"`
	ExecutionToolSurfacePresent bool              `json:"execution_tool_surface_present"`
	SDKToolSurfaces             map[string]string `json:"sdk_tool_surfaces,omitempty"`
}

func cacheSurfaceProfileFromOptions(ctx context.Context, options agentclient.Options) (CacheSurfaceProfile, error) {
	if err := ctx.Err(); err != nil {
		return CacheSurfaceProfile{}, err
	}
	profile := CacheSurfaceProfile{
		RuntimeKind:                 string(normalizedManagedRuntimeKind(options.Runtime.Kind)),
		ProviderFingerprint:         fingerprintCacheIdentity("provider", options.Env[runtimeProviderEnvName]),
		ModelFingerprint:            fingerprintCacheIdentity("model", options.Model),
		GoalToolSurfacePresent:      hasMCPServer(options, managedGoalMCPServerName),
		ExecutionToolSurfacePresent: hasMCPServer(options, managedExecutionMCPServerName),
		ManagedToolPresenceKnown:    strings.TrimSpace(options.MCP.Config) == "",
		HostToolSurfaceComplete:     true,
		ManagedToolSurfaceComplete:  true,
		ToolPolicyFingerprint:       fingerprintNativeToolSurface(options),
		MCPServersFingerprint:       fingerprintMCPServerSet(options),
	}

	serverSurfaces, managedComplete, hostComplete, err := hostSDKToolSurfaceFingerprints(ctx, options)
	if err != nil {
		return CacheSurfaceProfile{}, err
	}
	profile.HostToolSurfaceComplete = profile.HostToolSurfaceComplete && hostComplete
	profile.ManagedToolSurfaceComplete = managedComplete
	profile.ToolSurfaceFingerprint = fingerprintCacheValue(cacheToolSurfaceManifest{
		Version:                     1,
		NativeToolSurface:           fingerprintNativeToolSurface(options),
		ToolPolicyFingerprint:       profile.ToolPolicyFingerprint,
		MCPServersFingerprint:       profile.MCPServersFingerprint,
		GoalToolSurfacePresent:      profile.GoalToolSurfacePresent,
		ExecutionToolSurfacePresent: profile.ExecutionToolSurfacePresent,
		SDKToolSurfaces:             serverSurfaces,
	})
	if !profile.ManagedToolSurfaceComplete {
		profile.HostToolSurfaceComplete = false
	}
	return profile, nil
}

// ModelToolSurfaceFingerprint 返回宿主可证明的模型可见工具面脱敏指纹。
//
// 它不是 provider cache key；产品层只把它作为不支持会话内动态工具更新的
// runtime 的保守 resume/reset 栅栏。
func ModelToolSurfaceFingerprint(ctx context.Context, options agentclient.Options) (string, bool, error) {
	profile, err := cacheSurfaceProfileFromOptions(ctx, options)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(profile.ToolSurfaceFingerprint) == "" {
		return "", false, errors.New("runtime model tool surface fingerprint is empty")
	}
	return profile.ToolSurfaceFingerprint, profile.ManagedToolSurfaceComplete, nil
}

func fingerprintMCPServerSet(options agentclient.Options) string {
	servers := make(map[string]string, len(options.MCP.Servers)+len(options.MCP.SDKServers))
	for alias, server := range options.MCP.SDKServers {
		if server != nil {
			servers[strings.TrimSpace(alias)] = "sdk"
		}
	}
	for alias, config := range options.MCP.Servers {
		alias = strings.TrimSpace(alias)
		if alias == "" || config == nil {
			continue
		}
		switch config.(type) {
		case sdkmcp.SDKServerConfig:
			servers[alias] = "sdk"
		case sdkmcp.HTTPServerConfig:
			servers[alias] = "http"
		case sdkmcp.SSEServerConfig:
			servers[alias] = "sse"
		case sdkmcp.StdioServerConfig:
			servers[alias] = "stdio"
		default:
			servers[alias] = "unknown"
		}
	}
	return fingerprintCacheValue(map[string]any{
		"servers":       servers,
		"opaque_config": strings.TrimSpace(options.MCP.Config) != "",
	})
}

func fingerprintNativeToolSurface(options agentclient.Options) string {
	preset := ""
	if options.Tools.Preset != nil {
		preset = strings.TrimSpace(options.Tools.Preset.Preset)
	}
	return fingerprintCacheValue(map[string]any{
		"available": options.Tools.Available,
		"preset":    preset,
		"allow":     options.Tools.Allow,
		"deny":      options.Tools.Deny,
	})
}

func hostSDKToolSurfaceFingerprints(
	ctx context.Context,
	options agentclient.Options,
) (map[string]string, bool, bool, error) {
	servers := make(map[string]sdkmcp.SDKMCPServer)
	managedComplete := true
	hostComplete := strings.TrimSpace(options.MCP.Config) == ""
	for alias, server := range options.MCP.SDKServers {
		if server != nil {
			servers[alias] = server
		}
	}
	for alias, config := range options.MCP.Servers {
		if config == nil {
			continue
		}
		switch typed := config.(type) {
		case sdkmcp.SDKServerConfig:
			if typed.Instance != nil {
				servers[alias] = typed.Instance
			}
		default:
			delete(servers, alias)
			hostComplete = false
		}
	}

	aliases := make([]string, 0, len(servers))
	for alias := range servers {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	surfaces := make(map[string]string, len(aliases))
	for _, alias := range aliases {
		// Only Nexus-owned aliases are safe to invoke during configuration.
		if !strings.HasPrefix(strings.TrimSpace(alias), "nexus_") {
			hostComplete = false
			continue
		}
		result, ok, err := inspectSDKToolSurface(ctx, servers[alias])
		if err != nil {
			return nil, false, false, err
		}
		if !ok {
			managedComplete = false
			hostComplete = false
			continue
		}
		surfaces[alias] = fingerprintCacheValue(result)
	}
	return surfaces, managedComplete, hostComplete, nil
}

func inspectSDKToolSurface(
	ctx context.Context,
	server sdkmcp.SDKMCPServer,
) (result any, ok bool, err error) {
	select {
	case cacheSurfaceInspectionSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
		return nil, false, nil
	}

	type inspectionResult struct {
		result any
		ok     bool
	}
	inspectionCtx, cancel := context.WithTimeout(ctx, cacheSurfaceInspectionTimeout)
	defer cancel()
	resultCh := make(chan inspectionResult, 1)
	go func() {
		defer func() {
			<-cacheSurfaceInspectionSlots
			if recover() != nil {
				resultCh <- inspectionResult{}
			}
		}()
		response, handleErr := server.HandleMessage(inspectionCtx, map[string]any{
			"jsonrpc": "2.0",
			"id":      "cache-surface",
			"method":  "tools/list",
		})
		if handleErr != nil || response["result"] == nil {
			resultCh <- inspectionResult{}
			return
		}
		resultCh <- inspectionResult{result: response["result"], ok: true}
	}()
	select {
	case inspected := <-resultCh:
		return inspected.result, inspected.ok, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-inspectionCtx.Done():
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}
		return nil, false, nil
	}
}

func fingerprintCacheIdentity(kind string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fingerprintCacheValue(map[string]string{"kind": kind, "value": value})
}

func fingerprintCacheValue(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

// CacheSurface 返回当前 session 已成功采用的脱敏 runtime cache 切面。
func (m *Manager) CacheSurface(sessionKey string) (CacheSurfaceProfile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.sessions[strings.TrimSpace(sessionKey)]
	if state == nil || state.Closing || state.Client == nil {
		return CacheSurfaceProfile{}, false
	}
	return state.CacheSurface, true
}
