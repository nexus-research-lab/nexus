package agent

import "github.com/nexus-research-lab/nexus/internal/model/agent"

// Options 表示 Agent 运行时配置。
type Options = agent.Options

// Agent 表示对外 Agent 模型。
type Agent = agent.Agent

// CreateRequest 表示创建 Agent 请求。
type CreateRequest = agent.CreateRequest

// UpdateRequest 表示更新 Agent 请求。
type UpdateRequest = agent.UpdateRequest

// ValidateNameResponse 对齐当前校验协议。
type ValidateNameResponse = agent.ValidateNameResponse
