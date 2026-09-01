// Package automation 封装自动化域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers、页面 Agent 任务 CRUD（含领域创建意图重放、立即运行 request identity、Agent/Session 原子重绑、可选 configuration_version CAS，并拒绝新建/编辑 script）、owner-scoped 持久审批 API、review_required 删除的显式停止确认收尾，以及带配置版本/投递次数 fence 的未确认结果显式重投。
//   - failure.go：Automation HTTP 失败映射，保留既有状态码并只按领域证据声明数据影响。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automation
