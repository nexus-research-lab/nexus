// Package workspace 封装工作区域的 HTTP handlers。
//
// L2 | 父级: internal/handler（L1 见 AGENTS.md）
//
// 成员清单：
//   - handlers.go：Handlers 及 workspace 文件读写/上传路由；文件条件写入冲突和
//     修改前拒绝以 FailureCore not_applied 返回，无法证明提交结果的内部失败返回
//     unknown；整文件正文与非流式内置预览有硬上限，下载和 PDF 保留 HTTP Range，
//     不自动重放或更改 Agent/Session/path 身份。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
