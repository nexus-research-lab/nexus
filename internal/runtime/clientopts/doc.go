// Package clientopts 组装 SDK runtime client 的启动选项、provider、MCP servers 与环境。
//
// L2 | 父级: internal/runtime（L1 见 AGENTS.md）
//
// 成员清单：
//   - agent_client.go / runtime_env.go：client 选项、nxs/Claude
//     Skill 动态发现与显式停用投影、主模型配置解析、同 Provider 后台进度模型回退、provider 协议环境、
//     按 owner 锁定的 workspace/长期记忆环境、Control/Relay 与宿主秘密清理、nexuscfg / Agent-facing nexus
//     网页检索默认预授权（保留显式范围与 deny）、physical-round capability、按 runtime 隔离的模型上限环境、Provider 结果与 profile。
//   - nxs 子智能体定义固定注入，宿主控制路径隐藏原生 Agent schema，不随派生改变工具面。
//   - mcp_servers.go：严格解析 Agent 持久化 stdio/http/sse MCP 配置并在禁止覆盖内建及 GitHub 等 Connector 托管名称的前提下合并。
//   - web_search.go：runtime 自有的 WebSearch 配置与环境投影。
//   - log_runtime.go：runtime 日志选项。
//   - desktop_sandbox.go：实验桌面执行策略装配，分别要求命令、原生 Read/Write/Edit、Glob/Grep、本地图片、远程图片网络、macOS 显式 HTTP/SSE MCP 端点网络、Notebook、Skill、指令/compact 文件读取与受控设置写入能力，区分 Skill 读取根与显式写入挂载，保留 Full Access 路径。
//   - runtime_admission.go：认证转场到 Agent runtime admission 与强隔离要求的动态依赖边界。
//   - desktop_sandbox_live_test.go：显式 opt-in 的 macOS 真实第三方 Provider 验收；经当前装配分别验证 nxs 文件边界与 Claude Read/Edit + Bash 沙箱，不属于普通测试或发布验收。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
// auto 原样传给两种运行时；bridge 协商 nxs 能力并确认 Claude 原生模式。
// RequireProjectFiles 与其余桌面文件要求共同进入启动选项，不从旧上下文能力推断项目定义覆盖。
// RequireManagedPolicy 要求固定托管来源与执行前完整性，不推断普通配置或凭据已收口。
// RequireSettingsFiles 确认普通配置的受限读取和完整快照，凭据隔离与原子持久化仍独立验收。
// RequireSettingsWrites 确认 Config 更新复用同一受检来源并在写后强制替换 runtime；跨进程事务和持久回执仍独立验收。
// nxs Provider 与 AutoDream 唤醒的宿主所有权在所有环境合并后固定；ExtraEnv/ConfigurationEnv 不能撤销。
// macOS 的持久化与 Connector MCP 认证 helper 由独立必需执行能力接纳，端点授权不扩张 helper 网络。
// macOS stdio MCP 通过 RequireMCPStdio 独立要求受限进程，纳入替换指纹与有效策略回执。
package clientopts
