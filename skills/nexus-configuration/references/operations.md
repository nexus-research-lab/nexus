# Nexus 配置域与角色

先运行 `nexuscfg inspect`。本文件与返回的 `access` 或 `definition.operations` 不一致时，以 CLI 当前结果为准。

## 角色边界

| authority | 可管理范围 |
|---|---|
| `owner_main` | 主智能体私聊中的当前 owner 全局配置 |
| `agent_self` | 当前 Agent 自己的 profile、runtime、Skill、情绪与当前私聊标题 |
| `room_host` | 当前 Room、成员、协作策略、conversation，以及当前 Agent 的 Room 上下文情绪 |
| `room_member` | 只读当前 Room，并修改当前 Agent 自己的 Room 上下文情绪 |

target 不能扩大上述范围。普通 Agent 的 self operation 会由服务端固定到当前 Agent；Room operation 会固定到当前 Room。

## 配置域

| 配置域 | owner 主智能体 | 普通 Agent 私聊 | Room | 常见生效时机 |
|---|---|---|---|---|
| `preferences` | 读写 | 无 | 无 | 立即或下一轮 |
| `providers` | 管理私有 Provider | 只读可用模型目录 | 无 | 测试立即；runtime 下一轮 |
| `agents` | 管理 owner Agent | 修改自身 profile/runtime | 无 | 撤权立即；多数设置下一轮 |
| `emotion` | 当前 Agent | 当前 Agent | 当前 Agent 的 conversation 情绪 | 下一轮 |
| `channels` / `connectors` | 管理 owner 连接 | 无 | 无 | 立即或下一会话 |
| `skills` | 管理来源与 Agent 绑定 | 安装/卸载自身 Skill | 无 | 目录立即；runtime 下一轮 |
| `sessions` | owner 范围重命名/删除 | 只重命名当前私聊 | 无 | 立即 |
| `rooms` | 管理 owner Room | 无 | 群主管理当前 Room；成员只读 | 安全变更立即；提示下一轮 |
| `host` | 只读 | 无 | 无 | 外部变更并重启 |

`automation` 使用内置 `automation` Skill 与 round-scoped `nexus automation` CLI，`workspaces` 使用 `nexus-manager` / `nexusctl`，`goals` 使用内置 `goal-manager` Skill 与 round-scoped `nexus goal` CLI，`executions` 使用内置 `execution-orchestrator` Skill 与 round-scoped `nexus execution` CLI。

## 确认、秘密与失败

- plan 标记 `requires_confirmation` 时，等待用户明确同意后才为 apply 添加 `--confirm`。
- Agent 不执行 `--secrets-stdin`，也不把秘密写入 input；需要凭据时转到 Settings 或人工终端。
- revision 冲突时重新 inspect 和 plan；结果不确定时用 `--verify` 与 history 核对。
- runtime 重载失败时按结果恢复或保留旧 runtime，不把数据库已写入等同于功能可用。
