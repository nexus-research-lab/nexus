---
name: nexus-manager
description: 由 Nexus 主智能体查询或管理 owner scope 内的平台资源，包括用户、Agent、DM/Room、conversation、Session、其他 Agent workspace 与 Skill。产品配置、Goal、Execution 和 Automation 使用各自专用 Skill，不走本控制面。
---

# Nexus Manager

本 Skill 使用 `nexusctl` 控制面，不处理产品配置。Provider、Agent runtime options、偏好、Channel、Connector 与配置化 Skill 绑定使用 `nexus-configuration`；Goal、Execution、Automation 使用各自 round-scoped `nexus` domain。

## 执行契约

- 优先执行宿主注入的 `NEXUSCTL_COMMAND_PATH`；示例中的 `nexusctl` 代表该入口。Bash 使用 `"$NEXUSCTL_COMMAND_PATH" --json ...`，PowerShell 使用 `& "${env:NEXUSCTL_COMMAND_PATH}" --json ...`。只有宿主没有注入且 PATH 已提供正式二进制时才直接写 `nexusctl`。
- 不探测、覆盖或拼接作用域环境变量，不传隐藏的 `--scope-user-id` / `--global-scope`，不搜索源码或 `go run ./cmd/nexusctl`。当前 owner 与 workspace scope 只来自宿主。
- 普通 Agent 在强隔离下不能调用控制面；自己的 workspace 直接使用文件工具。其他资源管理请用户切换到主智能体，不通过数据库、state/runtime 文件或环境变量绕过。
- 所有调用带 `--json` 并作为单独进程执行。成功必须有 `success=true`，再读取 `domain`、`action` 与 `item|items|report`；失败读取结构化 `error.kind/message` 和退出码，不从旧字段或混合 stdout 猜结果。
- 先 list/get 当前状态，再执行一个用户授权的 mutation，最后重新读取验证。删除、卸载、移除成员、覆盖 workspace 或清理历史前精确核对目标和影响范围；没有明确删除意图时不执行破坏性命令。
- 低频参数只对选定的 exact subcommand 运行 `--help`，不要为发现能力加载其他 domain 或凭旧示例补 flag。

## 按资源读取参考

- auth、user 与 Agent：[references/accounts-and-agents.md](references/accounts-and-agents.md)
- DM/Room、conversation、Session 与消息历史：[references/rooms-and-sessions.md](references/rooms-and-sessions.md)
- 其他 Agent workspace 文件：[references/workspaces.md](references/workspaces.md)
- Skill 目录、安装、导入、更新与卸载：[references/skills.md](references/skills.md)

只读取当前资源 domain 的参考，不为一次调用加载完整 `nexusctl` 手册。

密码、token 和密钥不得出现在命令参数、日志、回复或临时文件。密码命令只在已有安全 stdin 通道时使用 `--password-stdin`；不要用 echo、管道、heredoc 或 shell history 传递，无法安全输入时让用户在人工终端或 Settings 完成。
