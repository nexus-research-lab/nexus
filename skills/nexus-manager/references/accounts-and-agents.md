# 账号与 Agent

只在管理认证用户或 Agent 资源时读取本文件。示例中的 `nexusctl` 代表主 Skill 规定的宿主入口，所有调用都加 `--json`。

## Auth 与 user

```bash
nexusctl --json auth status
nexusctl --json auth init-owner --username '<username>' --display-name '<name>' --password-stdin
nexusctl --json user list
nexusctl --json user create --username '<username>' --display-name '<name>' --role member --password-stdin
nexusctl --json user reset-password --username '<username>' --password-stdin
nexusctl --json user reset-password --user-id '<user-id>' --password-stdin
```

- `auth init-owner` 只用于系统尚无 owner 的初始化；先读 `auth status`，已有用户时不调用。
- create/reset 前先 `user list` 精确定位。reset 使用 username 或 user ID，不同时猜两个 locator。
- 密码只能通过安全 stdin 提供；不能安全提供时停止并交给人工终端。最终回复不复述密码。

## Agent

```bash
nexusctl --json agent list
nexusctl --json agent get '<agent-id>'
nexusctl --json agent create --name '<name>' --avatar '<avatar>' --description '<description>'
```

`agent create` 只有 name 必填；avatar/description 可选。创建前检查是否已有符合用户意图的 Agent，成功后返回服务端生成的 exact `agent_id`，后续命令使用 ID，不用显示名猜 locator。

Agent profile/runtime 的修改不属于当前 `nexusctl agent` surface；使用 `nexus-configuration` inspect 当前 `agents` domain，而不是编造 update/delete 子命令。
