// Package channel 封装 IM 通道域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及通道路由。
//   - control.go：通道控制 handler；当前 Web 登录 GET 只调用 service 只读
//     对账，不启动新扫码，空结果与歧义结果分别保持 not_found/conflict 读事实；
//     账号删除先按 URL path-segment 规则解码 account_id，再进入精确持久键删除。
//   - failure.go：配置、账号、登录与 Pairing 的 FailureCore 投影；只消费
//     service 明确阶段证据，unknown 只允许读取对账，不从 error 文本猜测。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package channel
