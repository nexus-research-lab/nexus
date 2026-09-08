# 账号与 Agent

只在管理认证用户或 Agent 资源时读取本文件。示例中的 `nexusctl` 代表主 Skill 规定的宿主入口，所有调用都加 `--json`。

## 用户账号

创建、修改和移除部署用户统一使用 `nexus-configuration` Skill 的 `members` 域，先执行 `"$NEXUSCFG_COMMAND_PATH" --json inspect --domain members`。`nexusctl` 不再提供 auth/user 子命令。

仅有效 owner/admin 登录的主智能体私聊可操作；移除表示撤销部署访问权限，保留用户和工作区数据。密码通过宿主确认卡片输入，不出现在命令或聊天里。若当前部署尚不支持 members，使用 Web 设置 → 运营 → 部署成员，不尝试旧二进制、数据库或隐藏 scope 参数。

## Agent

```bash
nexusctl --json agent list
nexusctl --json agent get '<agent-id>'
nexusctl --json agent create --name '<name>' --avatar '<avatar>' --description '<description>'
```

`agent create` 只有 name 必填；avatar/description 可选。创建前检查是否已有符合用户意图的 Agent，成功后返回服务端生成的 exact `agent_id`，后续命令使用 ID，不用显示名猜 locator。

Agent profile/runtime 的修改不属于当前 `nexusctl agent` surface；使用 `nexus-configuration` inspect 当前 `agents` domain，而不是编造 update/delete 子命令。
