// Package tool 定义 nexus_config 的四个稳定对话工具：inspect、plan、apply 与 history。
//
// L2 | 父级: internal/mcp/configuration
//
// plan 只生成绑定可信身份、资源 scope、版本与输入的 plan digest；apply 必须回传 digest、
// expected revision 与必要确认，history 只能读取当前动态权限允许的审计范围。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go
package tool
