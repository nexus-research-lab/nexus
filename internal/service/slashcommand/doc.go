// Package slashcommand 负责 Nexus host 侧 Slash 指令的注册与派发。
//
// runtime 指令仍由 nxs 或 Claude Code 所有，只通过 bridge 初始化快照进入 Nexus。
// 应在构造 WebSocket handler 前注册进程级 host registry；本包不依赖 runtime 内部实现。
package slashcommand
