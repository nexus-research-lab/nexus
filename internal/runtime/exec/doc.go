// Package exec 是轮次执行内核：ExecuteRound 主链（query → receive → map → persist → emit）。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - round.go：ExecuteRound 阶段编排与单轮接收状态。
//   - model.go：RoundExecutionRequest/Result、RoundMapper、RoundMapResult、ErrRoundInterrupted。
//   - stream_diagnostics.go / stream_error.go：流停止诊断、流关闭与空闲超时错误。
//   - terminal.go：终态判定与终态结果构造。
//   - util.go：轮次内容、空闲计时与中断辅助。
//   - aliases.go：复用 runtime 的 Client / ContextualInputBlock 类型别名。
//
// exec 单向依赖 runtime 核心（Client、ContextualInputBlock、若干导出函数）与 trace；
// runtime 核心不反向依赖 exec。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package exec
