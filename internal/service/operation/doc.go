// Package operation 管理操作舞台的恢复状态、在线实例与 Agent OS 工具路由。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：Service 装配与舞台快照持久化。
//   - stage_presence.go：显式舞台在线租约。
//   - stage_runtime_context.go：仅在舞台在线时注入 Agent 的 App 展示能力契约。
//   - stage_open_routing.go：舞台开启时的文件与网址打开命令路由。
//   - browser_page.go：Navi 公网页面抓取、防 SSRF 与嵌入页面重写。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package operation
