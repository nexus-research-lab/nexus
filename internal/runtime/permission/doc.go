// Package permission 处理 runtime 阻塞式人工交互的呈现与多 transport 响应。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - request.go：等待用户响应、按创建顺序和请求身份稳定重放，并向 Room 投影待确认状态的请求模型。
//   - presenter.go：批准、问答及未知工具的兼容呈现。
//   - context.go：Sender/Room 广播抽象、session 绑定、pending 变化信号与 owner lease 路由上下文。
//   - session_decision.go：Web 与 active-paired IM 共用的 session-scoped pending 查询/决策入口；无 ID 命令只命中唯一请求。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package permission
