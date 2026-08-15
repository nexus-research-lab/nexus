---
name: nexus-configuration
title: Nexus 配置
description: 在当前 Nexus 私聊或 Room 中安全读取、修改并验证调用者有权管理的产品配置。用户要求检查、启用、关闭、测试或修复 Agent 自身、Room、Provider、偏好、Channel、Connector、Skill、会话、模型、工具或 MCP 设置时使用。
scope: any
tags: [nexus, configuration, settings, agent, room]
---

# Nexus 配置

使用宿主注入的 `nexuscfg`。宿主会把命令绑定到当前 Agent、DM 或 Room round；配置服务根据真实身份返回 `owner_main`、`agent_self`、`room_host` 或 `room_member` 权限。不要自行声明 owner、Agent、Room 或 session。

## 命令入口

优先运行 `NEXUSCFG_COMMAND_PATH` 指向的命令，示例里的 `nexuscfg` 只是简写。所有 Agent 调用都加 `--json`。不要搜索源码入口、手写 `go run ./cmd/nexuscfg`、传 `--scope-user-id` / `--global-scope`，也不要覆盖 `NEXUSCFG_*` 环境变量。

## 工作流

1. 只 inspect 相关域；排障时加 `--verify`。

   ```bash
   nexuscfg --json inspect --domain agents --verify
   ```

2. 以返回的 `authority`、`access.allowed_operations`、`definition.operations`、`revision` 和 `checks` 为准。普通 Agent 可管理自己的安全子集；Room 权限只作用于当前 Room；只有主智能体私聊拥有 owner 全局能力。不要猜 operation、target 或输入字段。
3. 写入前运行 plan，参数中不得出现明文秘密。

   ```bash
   nexuscfg --json plan \
     --domain agents \
     --operation update_self_profile \
     --input '{"name":"新名称"}'
   ```

4. 向用户说明 `summary`、`risk`、`runtime_effect` 和确认要求。`requires_confirmation=true` 时等待用户明确同意；获得同意后才给 apply 加 `--confirm`。

   ```bash
   nexuscfg --json apply \
     --domain agents \
     --operation update_self_profile \
     --input '{"name":"新名称"}' \
     --expected-revision REVISION
   ```

5. 检查 apply 的写后验证；不确定时重新 inspect 或查询 history。revision 冲突时重新 inspect/plan，不覆盖新状态。

需要核对角色矩阵、域边界或生效时机时，读取 [references/operations.md](references/operations.md)。

## 秘密与授权

- 不向用户索取或在聊天、命令参数、文件、日志中写入 token、密码、私有 header、授权码或密钥。
- Agent 不使用 `--secrets-stdin`。需要 secret slot 时，引导用户在 Settings 或自己的人工终端完成。
- Connector OAuth/device 和 Channel 扫码、验证码继续使用对应的专用授权工具。

## 边界

- 不直接编辑 Nexus 数据库、状态文件、运行时环境或产品配置文件，也不用 `nexusctl` 代替 `nexuscfg`。
- `host` 只读；部署环境、启动参数和桌面状态根通过部署或桌面控制面修改。
- 权限被拒绝表示当前 Agent/DM/Room 没有该 operation。报告真实边界，不尝试切换 target、伪造身份或绕过 capability。

## 回复

简要说明改了什么、作用域、生效时机和验证结果。
