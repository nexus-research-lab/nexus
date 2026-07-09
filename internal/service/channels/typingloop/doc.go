// Package typingloop 在慢回复时延迟打开 typing 指示并按 IM 平台 TTL 周期续租。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单：
//   - typing_loop.go：Start 及 typing 续租循环。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package typingloop
