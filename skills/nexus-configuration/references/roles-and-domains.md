# Nexus 配置角色与 domain

只在选择 configuration domain、确认 role 边界或判断生效范围时读取本文件。先运行 current `inspect`；本文件与返回的 `access`、`definition.operations` 或 checks 不一致时，以 CLI 当前结果为准。

## Authority

| authority | 可管理范围 |
|---|---|
| `owner_main` | 主智能体私聊中的当前 owner 全局配置 |
| `agent_self` | 当前 Agent 自身 profile、runtime、Skill、情绪与当前私聊 Session |
| `room_host` | 当前 Room、成员、协作策略、conversation，以及当前 Agent 的 Room 上下文情绪 |
| `room_member` | 只读当前 Room，并修改当前 Agent 自己的 Room 上下文情绪 |

target 不能扩大 authority。普通 Agent 的 self operation 固定到当前 Agent；Room operation 固定到当前 Room。先从 inspect 的 `allowed_operations` 选择，再使用对应 definition 的 target/input，不从资源名称推断越权 operation。

## Domain 路由

| domain | 用途 | 常见生效时机 |
|---|---|---|
| `preferences` | owner 偏好 | 立即或下一轮 |
| `providers` | Provider、模型目录、默认模型与连接测试 | 测试立即；runtime 下一轮 |
| `agents` | Agent profile、runtime 与 owner Agent 管理 | 撤权立即；多数设置下一轮 |
| `emotion` | 当前 Agent 或当前 Room conversation 情绪 | 下一轮 |
| `channels` / `connectors` | owner 外部连接及授权状态 | 立即或下一会话 |
| `skills` | Skill 来源、目录与 Agent 绑定 | 目录立即；runtime 下一轮 |
| `sessions` | 当前 authority 可见的真实 Session 标题或删除 | 立即 |
| `rooms` | Room profile、成员、协作策略与 conversation | 安全变更立即；提示下一轮 |
| `host` | 脱敏启动配置和健康状态 | 只读；外部修改并重启 |

下列能力不属于 configuration domain：

- scheduled task / heartbeat：使用 `automation` Skill 与 round-scoped `nexus_runtime.command`。
- Goal：使用 `goal-manager` 与 round-scoped `nexus_runtime.command`。
- Execution/WorkGraph：使用 `execution-orchestrator` 与 round-scoped `nexus_runtime.command`。
- 其他 Agent workspace 与平台资源：仅主智能体使用 `nexus-manager` / `nexusctl`；当前 Agent 自己的 workspace 直接用文件工具。

## 失败与恢复

- plan 标记 confirmation 时，只针对展示的 normalized plan 取得用户确认；input 或 revision 改变后重新 plan/确认。
- revision 冲突回到 inspect/plan。runtime reload 失败时以 result/checks 决定重试、保留旧 runtime 或报告恢复动作，不把配置记录已写入当成功。
- 资源删除、权限撤销、Session/Room 变更可能影响运行中任务或连接；以 plan 的 risk/runtime effect 和写后 checks 为准，不靠本表预判。
