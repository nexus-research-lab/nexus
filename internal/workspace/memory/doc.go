// Package memory 把本地 markdown 记忆升级为可召回、可提交、可治理的运行时接口。
//
// L2 | 父级: internal/workspace（L1 见 AGENTS.md）
//
// 成员清单：
//   - engine*.go：Engine 记忆引擎（capture / query / mutation / render / message / scope_score）。
//   - repository*.go：记忆持久化（entry / file / search / session / context / checkpoint / cleanup）。
//   - parser.go / similarity.go / scheduler.go / service.go：解析、相似度、调度、服务入口。
//   - entry.go / model_engine.go / factory.go / errors.go：条目、模型、构造、错误。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package memory
