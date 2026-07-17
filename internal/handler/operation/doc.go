// Package operation 暴露操作舞台恢复、在线状态和 Navi 页面快照接口。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：舞台快照、在线租约与受控公网页面 handlers。
//   - handlers_test.go：状态生命周期和浏览器错误页契约测试。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package operation
