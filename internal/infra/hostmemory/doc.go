// Package hostmemory 读取宿主内存状态，供 app 注入 runtime 软准入。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：memory.go 提供探针、余量判断与默认会话额度；memory_darwin.go 使用内核压力信号；
// memory_linux.go 读取 MemAvailable；memory_windows.go 读取物理及提交余量；
// memory_other.go 对不支持的平台返回错误。
// 暴露接口：NewAdmissionProbe（读取失败时放行并报告）、DefaultSessionLimit；
// 不执行外部命令，不限制或终止现有进程。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package hostmemory
