// Package cli 装配 nexusctl、nexuscfg 与 Agent-facing nexus 命令行应用。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - app.go：New 创建单次执行并负责关闭延迟服务的 CLI 应用。
//   - config_environment.go：把 Agent runtime 目录视图还原为 nexusctl 宿主配置。
//   - services.go：按命令域延迟创建服务，避免 help 等命令拉起全量后端依赖。
//   - automation*.go：自动化任务的查询、变更、运行与运维命令；新投递目标绑定既有结构化 Session。
//   - runtime.go / runtime_automation.go：不打开数据库、只通过 physical-round
//     capability 调用宿主领域服务的 nexus CLI 与 Automation 子命令。
//   - skill.go / skill_agent.go / skill_external.go：技能目录、Agent 安装与来源管理命令。
//   - 其余领域文件：agent / auth / channel / connector / conversation（含空白会话维护）/ emotion /
//     imagegen / launcher / room / session / workspace 命令域。
//   - output.go / flag_int_pointer.go：输出格式与 flag helper。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package cli
