// Package tool 实现 nexus_connectors MCP 的工具集（含飞书文档能力）。
//
// L2 | 父级: internal/mcp/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - call.go / list.go：通用 connector 调用与列举。
//   - feishu_docx_client.go：飞书文档 API 客户端。
//   - feishu_docx_document.go / feishu_docx_sheet.go / feishu_docx_bitable.go /
//     feishu_docx_drive_wiki.go：文档 / 表格 / 多维表格 / 云盘知识库工具。
//   - registry.go / metadata.go / render.go：工具注册、元数据、结果渲染。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package tool
