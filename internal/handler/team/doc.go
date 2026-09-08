// Package team 暴露登录用户访问多人 Team 的 Nexus HTTP/WSS gateway。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：同源校验、Control 短令牌交换、Relay HTTP/WSS 转发、stream 换代提示、本地投影与稳定失败映射。
//
// 暴露接口：Handlers、New，以及 bootstrap、message、snapshot、difference、stream handlers。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package team
