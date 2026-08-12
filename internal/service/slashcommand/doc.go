// Package slashcommand 负责 Nexus host 侧 Slash 指令的注册、派发与版本化目录。
//
// runtime 指令仍由 nxs 或 Claude Code 执行，但 Composer 使用 Nexus 内置的稳定
// 清单。Catalog 不依赖 bridge 初始化结果或 DM/Room session，也不会启动 runtime。
// `/goal` 与 UI set_goal 共享同一 host handler，始终在 runtime 之前被截获。
package slashcommand
