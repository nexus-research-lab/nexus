// INPUT: HTTP/service 层 round cursor 与冷索引等待策略。
// OUTPUT: DM/Room 共用、无 positional 参数漂移的历史分页查询。
// POS: workspace history page 对上层暴露的唯一查询参数契约。
package workspace

type HistoryPageQuery struct {
	Limit                int
	BeforeRoundID        string
	BeforeRoundTimestamp int64
	AroundRoundID        string
	AroundLimit          int
	DeferIndex           bool
}
