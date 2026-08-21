# Room、conversation 与 Session

只在管理 DM/Room、话题、Session 或读取历史消息时读取本文件。所有 opaque ID 先从 list/get 结果取得，不从标题或正文猜。

## Room 与成员

```bash
nexusctl --json room list
nexusctl --json room get '<room-id>'
nexusctl --json room contexts '<room-id>'
nexusctl --json room ensure-dm --agent-id '<agent-id>'
nexusctl --json room create --agent-id '<agent-id>' --name '<name>' --title '<title>'
nexusctl --json room update '<room-id>' --name '<name>' --title '<title>'
nexusctl --json room add-member '<room-id>' --agent-id '<agent-id>'
nexusctl --json room remove-member '<room-id>' --agent-id '<agent-id>'
nexusctl --json room delete '<room-id>'
```

- create 的 `--agent-id` 和 `--skill-name` 可重复；至少一个 Agent 必须提供，主智能体不能作为 Room 成员。description/name/title/skills 只按用户意图传。
- update 只传实际修改的 flag。remove-member/delete 前先 get Room，核对 exact member、运行中责任和删除影响。
- 运行时 directed message 使用 communication 工具，不用控制面伪造发送。`room message list|cursors` 只读取 owner-scoped ledger；正文只有明确需要时才加 `--include-content`，`--after-cursor` 必须同时给 `--agent-id`。

## Conversation

```bash
nexusctl --json conversation list --room-id '<room-id>'
nexusctl --json conversation get '<conversation-id>'
nexusctl --json conversation create --room-id '<room-id>' --title '<title>'
nexusctl --json conversation update --room-id '<room-id>' --conversation-id '<conversation-id>' --title '<title>'
nexusctl --json conversation delete --room-id '<room-id>' --conversation-id '<conversation-id>'
nexusctl --json conversation messages --conversation-id '<conversation-id>'
```

Room 下也存在等价的 `create-conversation|update-conversation|delete-conversation` 位置参数入口；同一意图只选一个 command family，不重复 mutation。

`conversation prune-empty` 是维护命令，默认 dry-run。只有用户明确要求清理、已检查 report，并确认同一数据目录的 Nexus 服务已停止时才加 `--apply`；普通 conversation 删除不使用它。

## Session

```bash
nexusctl --json session list
nexusctl --json session list --agent-id '<agent-id>'
nexusctl --json session get --session-key '<session-key>'
nexusctl --json session create --session-key '<session-key>' --agent-id '<agent-id>' --title '<title>'
nexusctl --json session update --session-key '<session-key>' --title '<title>'
nexusctl --json session messages --session-key '<session-key>'
nexusctl --json session delete --session-key '<session-key>'
```

Session 使用完整 structured key；不要把裸 channel/chat/room ID 当 session key。创建、删除或改标题前先 list/get 并确认真实 Session。删除可能使引用的 Automation task 进入 rebind_required，先说明影响并取得明确删除意图。
