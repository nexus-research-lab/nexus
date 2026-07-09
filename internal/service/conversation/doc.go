// Package conversation 把应用层附件解析成当前 runtime 可读取的真实路径。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - attachment.go：AttachmentPathResolver 附件路径解析。
//
// 会话标题生成见子包 titlegen/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package conversation
