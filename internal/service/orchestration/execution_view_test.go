// 测试专用入口：生产路径已不再调用，仅供本包测试复用。
package orchestration

import "github.com/nexus-research-lab/nexus/internal/protocol"

// projectExecutionGraphView 是只给无 Repository 单测使用的窄入口。
func projectExecutionGraphView(
	items []protocol.ExecutionWorkItemView,
) protocol.ExecutionGraphView {
	return projectExecutionGraphViewWithHistory(items, protocol.ExecutionWorkGraphHistory{})
}
