// Package slashcommand 负责 Nexus Slash 指令的注册、派发、版本化目录与产品提示展开。
//
// runtime 指令仍由 nxs 或 Claude Code 执行，但 Composer 使用 Nexus 内置的稳定
// 清单。产品提示只在 runtime 投递边界展开，Catalog 不依赖 bridge 初始化结果或
// DM/Room session，也不会启动 runtime。
// `/goal` 与 UI set_goal 共享同一 host handler，始终在 runtime 之前被截获；
// `/workgraph` 只启用当前请求的 WorkGraph 协作，owner 命名工作图由
// service/workgraphworkflow 在同一 runtime 投递边界展开。
package slashcommand
