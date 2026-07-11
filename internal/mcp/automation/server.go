// Package automationmcp 提供 nexus_automation MCP server 入口。
//
// 子包分工：
//   - contract/   公共契约：Service 接口、ServerContext、ServerName
//   - tool/       MCP 工具（每个工具一个文件）+ JSON Schema，更新工具按字段阶段构建输入
//   - internal/argx/      入参类型转换与访问器
//   - internal/render/    返回值序列化 + 时间字段本地化
//   - internal/builder/   schedule/session/delivery/source 对象构造
//   - internal/semantic/  页面语义 → 底层结构的翻译、校验与默认值守卫
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automationmcp

import (
	sdktool "github.com/nexus-research-lab/nexus/internal/mcp/sdktool"

	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/tool"
)

// NewServer 根据当前会话上下文构建 nexus_automation MCP server。
func NewServer(svc contract.Service, sctx contract.ServerContext) *sdktool.SimpleSDKMCPServer {
	return sdktool.NewSimpleSDKMCPServer(contract.ServerName, "1.0.0", tool.BuildAll(svc, sctx))
}
