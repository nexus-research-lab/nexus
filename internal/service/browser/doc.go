// Package browser 管理 Nexus 浏览器扩展连接、会话标签页归属与命令回执。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：单扩展连接、并发请求路由、扩展代次与不透明标签页引用栅栏、子标签页继承、按 Session 隔离的多标签页状态、Browser action 清单与输入校验。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package browser
