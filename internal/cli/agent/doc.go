// Package agent 装配 Agent-facing 的 round-scoped nexus CLI，供 nxs/Claude
// runtime 经宿主签发的 physical-round capability 调用领域服务。
//
// L2 | 父级: internal/cli（L1 见 AGENTS.md）
//
// 信任边界：与 internal/cli/host（nexusctl/nexuscfg 宿主 CLI）分离。本包不装配
// AppServices、不打开数据库、不接受 owner/Agent/Session/权限覆盖，只依赖
// internal/cli/runtimecommand 的 command wire 与宿主签发的 broker capability。
//
// 成员清单：
//   - runtime.go：RunRuntime 入口、nexusctl/nexus-server multicall 私有入口识别与 nexus 根命令。
//   - output.go：Agent 进程私有的输出 envelope、错误分类与日志开关。
//   - runtime_semantic.go：goal / execution / computer 命令族；自描述 command_usage 与 contract、inspect、invoke 单层 typed JSON envelope。
//   - runtime_automation.go：automation 命令族；自描述 inspect/plan/apply/verify transport。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
