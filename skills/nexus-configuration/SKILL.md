---
name: nexus-configuration
title: Nexus 配置
description: 在当前 Nexus 私聊或 Room 中读取、规划、确认并验证调用者有权管理的产品配置，包括管理员的用户账号、Agent、Room、Provider、偏好、Channel、Connector、Skill、Session、模型、工具和 MCP 设置。
scope: any
tags: [nexus, configuration, settings, agent, room]
---

# Nexus 配置

配置命令使用宿主注入的 `NEXUSCFG_COMMAND_PATH`；示例中的 `nexuscfg` 只代表该入口。宿主绑定当前 Agent、DM/Room round 与 owner scope，服务端返回真实 `owner_main|agent_self|room_host|room_member` authority。不要声明、切换或覆盖 identity/scope，也不要使用 owner 控制面、数据库或配置文件替代本能力。

## 固定生命周期

所有 mutation 固定走 `inspect → plan → apply → verify`。

1. 只 inspect 相关 domain；排障时加 `--verify`：

   ```bash
   "$NEXUSCFG_COMMAND_PATH" --json inspect --domain agents --verify
   ```

   PowerShell 使用 `& "${env:NEXUSCFG_COMMAND_PATH}" ...`，不要混用 shell 变量语法。

2. 以顶层 `inspection` 中的 `authority`、`access.allowed_operations`、`definition.operations`、`revision` 与 checks 为准。不要根据 Skill 猜 operation、target 或 input；需要角色与 domain 分流时读取 [references/roles-and-domains.md](references/roles-and-domains.md)。
3. mutation 先用同一 domain/operation/target/input 执行 plan。输入必须是一个不含秘密的 JSON object：

   ```bash
   "$NEXUSCFG_COMMAND_PATH" --json plan --domain agents --operation update_self_profile --input '{"name":"新名称"}'
   ```

4. 核对 plan 的 normalized change、summary、risk、runtime effect、`current_revision`、`plan_digest` 与 confirmation/secret slots。`requires_confirmation=true` 时等待用户针对该 plan 明确同意；只有随后 apply 才加 `--confirm`。
5. apply 保持同一 change，携带 plan revision 与稳定 request ID；revision 冲突时回到 inspect/plan，不覆盖新状态：

   ```bash
   "$NEXUSCFG_COMMAND_PATH" --json apply --domain agents --operation update_self_profile --input '{"name":"新名称"}' --expected-revision '<revision>' --request-id 'config-agent-profile-UNIQUE'
   ```

6. 读取顶层 `result` 的写后 checks；不确定时重新 inspect `--verify` 或用 `history --domain '<domain>'` 核对。数据库已写入不等于 runtime 已生效，以返回的 runtime effect 和验证结果为准。

## 秘密与权限

- 不向用户索取或在聊天、命令参数、文件、日志中写入 token、密码、私有 header、授权码或密钥。Agent 永不使用 `--secrets-stdin`；`members.create` 使用 `{"$secret":"member-password"}` 占位，apply 由宿主确认卡片收集密码；其他域出现 secret slot 时，引导用户在 Settings 或人工终端完成。
- Connector OAuth/device 与 Channel 扫码、验证码继续使用对应专用授权流程，不把凭据塞进通用 config input。
- permission denied 表示当前 Agent/DM/Room 没有该 operation。报告真实边界，不换 target、不伪造身份，也不传隐藏的 `--scope-user-id` / `--global-scope`。
- `host` 只读；部署环境、启动参数和桌面状态根通过部署或原生桌面控制面修改。

回复简要说明真实变更、作用域、生效时机和验证结果；不要输出脱敏前配置、capability 或完整审计载荷。

## 管理用户

管理员需要新增、修改或移除用户时，读取 [references/members.md](references/members.md)。不要调用旧 `nexusctl user/auth`，不要操作 Control 数据库或索取服务令牌。

## 创建 Agent 与行为模板

主智能体创建普通 Agent 时，先读取 `nexus-manager` 的 `references/accounts-and-agents.md`，按其中的头像选择与行为模板补全流程完成。若通过本 Skill 的 `agents.create` 创建，仍走上述配置生命周期，并显式设置头像和介绍；未读取真实默认模板时省略 `profile_template`，让宿主生成默认文件，再用返回的 exact Agent ID 读取、补全并验证 `AGENTS.md`。不要把简短介绍或新写的人设段落作为完整 `profile_template` 覆盖默认内容。

## 修改已有 Agent 的行为模板

`profile_template` 是创建时初始化 workspace 根级 `AGENTS.md` 的输入。创建后补充、修改或重写角色职责与工作方式，编辑该文件；名称、头像、目录摘要与 runtime 配置使用 `agents` 配置域。

- 修改自己的模板：使用原生文件工具读取、编辑并读回 `AGENTS.md`。
- 修改其他 Agent 的模板：由主智能体读取 `nexus-manager` 的 `references/workspaces.md`，按 exact Agent ID 读取、更新并读回 `AGENTS.md`；普通 Agent 请用户切换到主智能体。

编辑以基础模板为底，填写或按用户要求调整 `Role` 中的 `Purpose`、`Responsibilities`、`Out of scope`、`Preferred working style`，也可按需新增角色字段或专用规则。保留 `Baseline Rules` 标题及全部基础规则，不删除、替换或用新增规则抵消它们；重写角色描述也只作用于相应角色内容。写回完整文件后读回，核对角色内容与基础规则。读回只能证明文件已保存，不能据此宣称活跃 runtime 已加载新规则。
