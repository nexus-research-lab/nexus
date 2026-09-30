// INPUT: 宿主审批生成的精确域名集合及本次启动 context。
// OUTPUT: 固定 HTTPS/443 公网地址清单，空集合拒绝全部网络，失败不回退 DNS。
// POS: App 到 Windows supervisor 的可信网络投影；SDK 仍独立检查每次真实拨号。
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"time"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

// resolveWindowsSandboxEndpoints 不解析任务 URL 或代理变量；审批变更需新执行。
func resolveWindowsSandboxEndpoints(ctx context.Context, domains []string) ([]runtimectx.WindowsSandboxNetworkEndpoint, error) {
	if len(domains) > 64 {
		return nil, errors.New("Windows sandbox domain admission exceeds launch limit")
	}
	canonical, err := (&clientopts.DesktopSandboxNetworkAdmission{AllowedDomains: domains}).CanonicalAllowedDomains()
	if err != nil {
		return nil, err
	}
	if len(canonical) == 0 {
		return []runtimectx.WindowsSandboxNetworkEndpoint{}, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("inspect host interfaces before sandbox DNS: %w", err)
	}
	local := make(map[netip.Addr]bool)
	for _, item := range interfaces {
		prefix, err := netip.ParsePrefix(item.String())
		if err != nil {
			return nil, errors.New("cannot establish host interface boundary")
		}
		local[prefix.Addr().Unmap()] = true
	}
	endpoints := make([]runtimectx.WindowsSandboxNetworkEndpoint, 0, len(canonical))
	for _, domain := range canonical {
		addresses, err := net.DefaultResolver.LookupNetIP(bounded, "ip", domain)
		if err != nil {
			return nil, fmt.Errorf("resolve approved Windows sandbox domain %q: %w", domain, err)
		}
		if len(addresses) == 0 || len(addresses) > 64 {
			return nil, fmt.Errorf("approved domain %q has an empty or oversized address set", domain)
		}
		fixed := make([]string, 0, len(addresses))
		for _, address := range addresses {
			address = address.Unmap()
			if !windowsSandboxPublicAddress(address) || local[address] {
				return nil, fmt.Errorf("approved domain %q resolves outside the public network boundary", domain)
			}
			fixed = append(fixed, address.String())
		}
		slices.Sort(fixed)
		fixed = slices.Compact(fixed)
		endpoints = append(endpoints, runtimectx.WindowsSandboxNetworkEndpoint{Scheme: "https", Host: domain, Port: 443, Addresses: fixed})
	}
	return endpoints, bounded.Err()
}

// windowsSandboxPublicAddress 与 SDK 公网合同一致，排除元数据、特殊用途和过渡地址。
// 此投影不能替代 SDK 真实拨号前再次核验本机地址及固定目标。
func windowsSandboxPublicAddress(address netip.Addr) bool {
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.Zone() != "" {
		return false
	}
	for _, prefix := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "168.63.129.16/32",
		"::/96", "64:ff9b::/96", "64:ff9b:1::/48", "2001::/32", "2001:db8::/32", "2002::/16",
	} {
		if netip.MustParsePrefix(prefix).Contains(address) {
			return false
		}
	}
	return !address.Is6() || netip.MustParsePrefix("2000::/3").Contains(address)
}
