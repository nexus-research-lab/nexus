// Package main summarizes local Nexus command diagnostics without executing commands.
//
// L2 | 父级: scripts（L1 见 AGENTS.md）
//
// 成员清单：
//   - main.go：JSON 日志配对、缺失/冲突统计、重复请求观察与宿主时长分布。
//   - main_test.go：实际 command 诊断兼容、日志重复、缺少结束、冲突和缺失值。
//
// 入口：go run ./scripts/workgraph-command-report --label NAME --environment DESCRIPTION LOG.jsonl ...
// 输出仅为诊断，不授予业务成功、幂等重放或单工具 token 归因。
//
// [PROTOCOL]: 变更时更新此头部，然后检查 AGENTS.md（L1）
package main
