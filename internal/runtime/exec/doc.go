// Package exec 是轮次执行内核：ExecuteRound 主链（query → receive → map → persist → emit）。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - round.go：ExecuteRound 阶段编排与单轮接收状态；人工交互期间暂停空闲计时，
//     显式中断后以 wire result 排空共享流，本地后处理失败时保留已观察到的 terminal usage。
//   - model.go：RoundExecutionRequest/Result、RoundMapper、RoundMapResult、runtime 类型别名与 ErrRoundInterrupted。
//   - stream_diagnostics.go / stream_error.go：流停止诊断、流关闭与空闲超时错误，
//     以及隔离内部诊断与会话展示文案的投影入口。
//   - terminal.go：终态判定与终态结果构造。
//   - util.go：轮次内容、空闲计时与中断辅助。
//
// exec 单向依赖 runtime 核心（Client、ContextualInputBlock、若干导出函数）与 trace；
// runtime 核心不反向依赖 exec。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package exec
