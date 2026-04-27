// Package automationmcp 提供 nexus_automation MCP server 入口。
//
// 子包分工：
//   - contract/   公共契约：Service 接口、ServerContext、ServerName
//   - tool/       8 个 MCP 工具（每个工具一个文件）+ JSON Schema
//   - internal/argx/      入参类型转换与访问器
//   - internal/render/    返回值序列化 + 时间字段本地化
//   - internal/builder/   schedule/session/delivery/source 对象构造
//   - internal/semantic/  页面语义 → 底层结构的翻译、校验与默认值守卫
package automationmcp

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"

	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/tool"
)

// NewServer 根据当前会话上下文构建 nexus_automation MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *agentclient.SimpleSDKMCPServer {
	return agentclient.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
