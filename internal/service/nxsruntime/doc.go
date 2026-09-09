// Package nxsruntime 报告 nxs runtime 在当前主机上的可用状态。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：Status 直接查询 bridge 本地探测器并转换为 RuntimeStatus，不持有状态、不触发下载。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package nxsruntime
