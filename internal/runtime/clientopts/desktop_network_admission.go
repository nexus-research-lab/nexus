// INPUT: 宿主准备的桌面 nxs 网络域名准入和已解析的 Agent MCP 配置。
// OUTPUT: 仅允许已核准 HTTPS 域名的 sandbox 网络配置与远程 MCP。
// POS: 桌面 runtime 的网络准入边界；环境变量清理不能替代这里的 OS/Bridge 策略。
package clientopts

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// DesktopSandboxNetworkAdmission is a host-prepared, exact-domain grant.
// The empty value is intentional: desktop nxs starts with network deny-all.
// User settings and task settings must not construct this object directly;
// production callers should obtain it from the host approval/reconciliation
// path and replace the runtime when the grant changes.
type DesktopSandboxNetworkAdmission struct {
	AllowedDomains []string
}

// CanonicalAllowedDomains validates and returns a stable, detached domain list.
func (a *DesktopSandboxNetworkAdmission) CanonicalAllowedDomains() ([]string, error) {
	if a == nil {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(a.AllowedDomains))
	for _, raw := range a.AllowedDomains {
		domain := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if err := validateDesktopSandboxDomain(domain); err != nil {
			return nil, err
		}
		seen[domain] = struct{}{}
	}
	domains := make([]string, 0, len(seen))
	for domain := range seen {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return domains, nil
}

func validateDesktopSandboxDomain(domain string) error {
	if domain == "" || len(domain) > 253 || strings.ContainsAny(domain, "/:@[]*?%#") {
		return fmt.Errorf("desktop sandbox network domain must be a bare DNS name")
	}
	if net.ParseIP(domain) != nil {
		return fmt.Errorf("desktop sandbox network domain must not be an IP address: %q", domain)
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("desktop sandbox network domain is invalid: %q", domain)
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("desktop sandbox network domain is invalid: %q", domain)
			}
		}
	}
	return nil
}

// Allows reports whether rawURL is an HTTPS URL to one exact approved domain.
// Explicit non-443 ports, userinfo, IP literals and private network addresses
// remain denied; a future host adapter can define a separate local grant.
func (a *DesktopSandboxNetworkAdmission) Allows(rawURL string) bool {
	if a == nil {
		return false
	}
	domains, err := a.CanonicalAllowedDomains()
	if err != nil {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if net.ParseIP(host) != nil {
		return false
	}
	for _, domain := range domains {
		if host == domain {
			return true
		}
	}
	return false
}

// DesktopSandboxNetworkConfig converts a host grant into an explicit Bridge
// network object. A nil/empty grant still serializes allowedDomains=[] and
// deniedDomains=[], preserving the SDK's deny-all presence semantics.
func (a *DesktopSandboxNetworkAdmission) DesktopSandboxNetworkConfig() (*agentclient.SandboxNetworkConfig, error) {
	domains, err := a.CanonicalAllowedDomains()
	if err != nil {
		return nil, err
	}
	return &agentclient.SandboxNetworkConfig{AllowedDomains: domains}, nil
}

// RejectDesktopSandboxRemoteMCPWithNetworkAdmission is the host-approved
// variant of RejectDesktopSandboxRemoteMCP. Without a grant, all persisted
// HTTP/SSE MCP remains rejected in restricted desktop nxs sessions.
func RejectDesktopSandboxRemoteMCPWithNetworkAdmission(
	configured map[string]any,
	runtimeKind string,
	appMode string,
	desktopSandboxEnabled bool,
	permissionMode sdkpermission.Mode,
	admission *DesktopSandboxNetworkAdmission,
) error {
	if admission == nil {
		return RejectDesktopSandboxRemoteMCP(configured, runtimeKind, appMode, desktopSandboxEnabled, permissionMode)
	}
	if !desktopSandboxEnabled || !strings.EqualFold(strings.TrimSpace(appMode), "desktop") ||
		!runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	if _, err := admission.CanonicalAllowedDomains(); err != nil {
		return fmt.Errorf("desktop sandbox network admission: %w", err)
	}
	for _, name := range sortedConfiguredMCPNames(configured) {
		object, ok := configured[name].(map[string]any)
		if !ok {
			continue
		}
		if helper, _ := object["headersHelper"].(string); strings.TrimSpace(helper) != "" {
			return agentMCPServerError(name, "桌面沙箱当前拒绝未受宿主管理的 MCP headers helper；需要受信任 helper 准入")
		}
		serverType, _ := object["type"].(string)
		serverType = strings.ToLower(strings.TrimSpace(serverType))
		if permissionMode == sdkpermission.ModeBypassPermissions || (serverType != "http" && serverType != "sse") {
			continue
		}
		serverURL, _ := object["url"].(string)
		if !admission.Allows(serverURL) {
			return agentMCPServerError(name, "桌面沙箱拒绝未获宿主域名准入的外部 HTTP/SSE MCP")
		}
	}
	return nil
}

// applyDesktopSandboxNetworkAdmission replaces the copied network object only
// after applyDesktopSandbox has installed the mandatory nxs contract.
func applyDesktopSandboxNetworkAdmission(options agentclient.Options, input AgentClientOptionsInput) (agentclient.Options, error) {
	if !input.DesktopSandboxEnabled || !strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop") || options.Runtime.Kind != agentclient.RuntimeNXS {
		return options, nil
	}
	network, err := input.DesktopSandboxNetworkAdmission.DesktopSandboxNetworkConfig()
	if err != nil {
		return agentclient.Options{}, err
	}
	if options.Sandbox == nil {
		return agentclient.Options{}, fmt.Errorf("desktop sandbox network admission requires sandbox settings")
	}
	options.Sandbox.Network = network
	return options, nil
}
