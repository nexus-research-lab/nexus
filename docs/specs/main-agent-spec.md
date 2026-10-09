# 主智能体规范

主智能体是每个 owner 的默认控制面 Agent，不是全局唯一的系统账号。

## 1. 身份与归属

- 身份真相源是当前 owner 下激活的 `is_main` Agent 记录；服务端初始化时保证该记录及其 workspace 可用。
- `is_main` 只能由服务端持久化记录和可信运行时上下文产生。
- 系统 owner 可沿用部署配置的默认 Agent ID；其他 owner 使用稳定的 owner-scoped Agent ID。业务代码不得依赖具体字符串。

## 2. Workspace 与运行时

主智能体与普通 Agent 使用同一套 owner 隔离布局：

```text
<users_root>/<owner_segment>/workspace/<agent_id>/
```

- 主智能体保留宿主控制面身份，但只能操作当前 owner scope，不获得跨 owner 的隐式管理权限。
- 普通 Agent 不因与主智能体共享 provider、模型或 Skill 而获得控制面权限。

## 3. 入口与 Room 边界

- 默认入口使用标准 Agent DM session，不新增专用会话协议；其运行时、历史和恢复规则与其他 Agent DM 一致。
- `/nexus/v1/runtime/options` 返回当前 owner 的 `default_agent_id`、头像和默认模型偏好；前端只消费该结果，不维护第二份默认 ID。
- 主智能体不能作为多人 Room 的普通成员，也不充当其成员模板。

## 4. 平台能力

- 默认加载普通基础 Skill，并额外加载 `nexus-manager`。
- 需要主智能体权限的 MCP、配置、通道或 Connector 授权能力，必须同时校验当前 owner、Agent 记录、会话类型和可信 `is_main` 上下文，不能只检查工具是否出现在 runtime 中。
- 职责：默认入口对话；当前 owner 范围内的 Nexus 配置与组织动作；需要可信控制面身份的平台能力。
- 不负责代替专业 Agent 执行具体协作。

## 5. 禁止项

- 通过 `"nexus"`、`"main"`、名称或头像判断主智能体。
- 由前端、模型参数、Agent 名称或展示文案设置 `is_main` 或把普通 Agent 提升为主智能体。
- 使用部署级默认 Agent ID 代替当前 owner 的主智能体查询。
- 把主智能体控制面能力公开成所有 Agent 都可调用的普通能力。
