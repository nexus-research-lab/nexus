// Package contract 定义 nexus MCP 图片生成子包共享的服务契约与上下文。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package contract

import (
	"context"

	imagegensvc "github.com/nexus-research-lab/nexus/internal/service/imagegen"
)

// ServerContext 承载当前 Agent 运行时上下文。
type ServerContext struct {
	OwnerUserID   string
	WorkspacePath string
}

// Service 是 nexus MCP 图片生成工具依赖的服务子集。
type Service interface {
	GenerateImage(ctx context.Context, input imagegensvc.GenerateInput) (*imagegensvc.Result, []byte, error)
	EditImage(ctx context.Context, input imagegensvc.EditInput) (*imagegensvc.Result, []byte, error)
}
