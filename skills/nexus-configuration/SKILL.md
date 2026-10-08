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

2. 以顶层 `inspection` 中的 `authority`、`access.allowed_operations`、`definition.operations`、`revision` 与 checks 为准。操作列表已按当前身份和 DM/Room 场景过滤，只描述本次调用者可用的配置能力；其他身份或专用入口按 [references/roles-and-domains.md](references/roles-and-domains.md) 分流。不要根据 Skill 猜 operation、target 或 input。
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

## 配置请求失败

`请求参数错误` / HTTP 400 本身不能证明权限不足、配置 API 缺失或需要升级。先核对宿主入口、子命令和参数；若多个配置域的只读 inspect 都同样失败，报告共同的配置请求链路异常及尚未确定的原因，提供失败命令、错误码和发生时间供维护者查日志。只有接口定义、版本或日志明确证明能力缺失时才建议升级；不要把 nexusctl 正常视为 nexuscfg 正常，也不要编造成功的 inspection。

## 管理用户

平台管理员需要新增、修改或停用独立 Web 用户时，读取 [references/members.md](references/members.md)。不要调用旧 `nexusctl user/auth`，不要操作 Control 数据库或索取服务令牌。

## Skill 内容与长期记忆

修改本地 Skill 的正文或脚本时，读取 [references/skill-content.md](references/skill-content.md)，按来源与所属 Agent 选择文件编辑入口。

长期记忆的内容位于 Agent workspace 的 `MEMORY.md` 与 `memory/`。自己的记忆使用原生文件工具读写；主智能体修改其他 Agent 的记忆按 `nexus-manager` 的 workspace 入口读取、编辑并读回。用户也可在联系人 → 选择智能体 → 记忆中编辑，页面保存携带读取 revision。修改前读取当前内容并保留无关信息；文件保存与运行中模型的记忆加载分别核对。

## Agent 创建与行为模板


创建 Agent 或修改已有 Agent 的角色职责、工作方式时，先读取 [references/agent-profile-template.md](references/agent-profile-template.md)，按创建或编辑流程处理 workspace 根级 `AGENTS.md`，保留基础规则并读回核对。名称、头像、目录摘要与 runtime 配置使用 `agents` 配置域。
