// Package contract 定义 nexus_imagegen MCP 子包共享的服务契约与上下文。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package contract

import (
	"context"

	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

// ServerName 是图片生成内建 MCP server 的注册名。
const ServerName = "nexus_imagegen"

// ServerContext 承载当前 Agent 运行时上下文。
type ServerContext struct {
	OwnerUserID   string
	WorkspacePath string
}

// Service 是 nexus_imagegen MCP server 依赖的图片生成服务子集。
type Service interface {
	GenerateImage(ctx context.Context, input imagegensvc.GenerateInput) (*imagegensvc.Result, []byte, error)
	EditImage(ctx context.Context, input imagegensvc.EditInput) (*imagegensvc.Result, []byte, error)
}
