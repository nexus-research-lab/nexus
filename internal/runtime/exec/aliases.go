package exec

import runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

// exec 复用 runtime 定义的这两个类型：Client 是 round 执行所操作的最小 SDK 能力，
// ContextualInputBlock 是 runtime 拥有、注入到本轮的隐藏上下文。用类型别名桥接，
// 避免把这两个横跨 runtime/exec 的类型强行搬家或双向依赖。
type (
	Client               = runtimectx.Client
	ContextualInputBlock = runtimectx.ContextualInputBlock
)
