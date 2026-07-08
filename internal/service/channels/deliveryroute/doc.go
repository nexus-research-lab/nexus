// Package deliveryroute 记录并读取每个会话最近一次成功投递的显式目标。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - store.go：投递路由读写（GetLastRoute 等）。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package deliveryroute
