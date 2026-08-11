# 主智能体规范

## 1. 文档目标

本文定义 Nexus 当前主智能体的身份、作用域、入口与能力边界。

## 2. 身份与归属

主智能体是每个 owner 的默认控制面 Agent，不是全局唯一的系统账号。服务端以当前
owner 下激活的 `is_main` Agent 记录为身份真相源，并在初始化时保证该记录及其
workspace 可用。

约束：

- `is_main` 只能由服务端持久化记录和可信运行时上下文产生。
- 前端、模型参数、Agent 名称或展示文案都不能把普通 Agent 提升为主智能体。
- 系统 owner 可以沿用部署配置的默认 Agent ID；其他 owner 使用稳定的 owner-scoped
  Agent ID。业务代码不得依赖具体字符串。

## 3. Workspace 与运行时

主智能体与普通 Agent 使用同一套 owner 隔离布局：

```text
<users_root>/<owner_segment>/workspace/<agent_id>/
```

主智能体保留宿主控制面身份，但只能操作当前 owner scope。普通 Agent 不会因为与
主智能体共享 provider、模型或 Skill 而获得控制面权限。

## 4. 入口与 Room 边界

- 默认入口使用标准 Agent DM session，不新增主智能体专用会话协议。
- `/nexus/v1/runtime/options` 返回当前 owner 的 `default_agent_id`、头像和默认模型偏好；
  前端只消费该结果，不维护第二份默认 ID。
- 主智能体不能作为多人 Room 的普通成员。
- `dm` 仍可由主智能体承接；其运行时、历史和恢复规则与其他 Agent DM 一致。

## 5. 平台能力

主智能体默认加载普通基础 Skill，并额外加载 `nexus-manager`。需要主智能体权限的
MCP、配置、通道或 Connector 授权能力必须同时校验当前 owner、Agent 记录、会话类型
和可信 `is_main` 上下文，不能只检查工具是否出现在 runtime 中。

主智能体负责：

- 默认入口对话。
- 当前 owner 范围内的 Nexus 配置与组织动作。
- 需要可信控制面身份的平台能力。

主智能体不负责：

- 代替所有专业 Agent 执行具体协作。
- 充当多人 Room 的普通成员模板。
- 获得跨 owner 的隐式管理权限。

## 6. 禁止项

- 通过 `"nexus"`、`"main"`、名称或头像判断主智能体。
- 由前端或模型参数设置 `is_main`。
- 使用部署级默认 Agent ID 代替当前 owner 的主智能体查询。
- 把主智能体控制面能力公开成所有 Agent 都可调用的普通能力。

一句话：主智能体是 owner-scoped 的可信控制面 Agent，身份由服务端记录决定，入口仍走标准 DM，权限始终受当前 owner 边界约束。
