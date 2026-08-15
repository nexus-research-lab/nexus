// Package automationmcp 提供 nexus_automation 内建 MCP server 入口。
// 模型通过 automation_query 与 automation_update 两个稳定入口访问自动化能力，
// 具体 operation 的选择与工作流由系统内置 automation Skill 说明。
// 交互 Nexus DM/Room Session 保持稳定工具表，但只有可信轮次能执行写工具；
// scheduled task 使用 request_id 创建幂等、
// configuration_version CAS 与写后重读，heartbeat 也按 Agent 权限提供读取、CAS
// 更新和独立 wake 动作；create/update 同步暴露 SDK permission_mode。普通 Agent 与 Room 内主智能体只能控制自身，owner main
// 仅在自己的私有 DM 获得跨 Agent authority。独立后台 profile 只暴露按当前
// Agent/外部会话收窄的诊断工具。普通 create/update schema 只让模型表达
// context_mode=current|isolated 与 deliver_result；当前会话由宿主从结构化 ServerContext
// 自动绑定，模型参数不能覆盖。owner main 的高级控制只能选择真实 Nexus/Room/IM
// Session，不提供合成 Agent 收件箱；旧 execution/reply 枚举只用于存量任务兼容。
// 宿主 script 任务始终保留在人类控制面。
//
// L2 | 父级: internal/mcp（L1 见 AGENTS.md）
//
// 成员清单：
//   - server.go：NewServer 按当前会话上下文构建定时任务 MCP server。
//   - contract/：服务依赖与会话上下文契约。
//   - tool/：只读查询与受信任变更两个模型工具，以及复用的内部操作 handler。
//   - internal/：参数、构建、语义默认值和结果渲染。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package automationmcp
