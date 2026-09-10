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

`agent create` 的协议只有 name 必填，但代用户创建普通 Agent 时，应一并设置头像、简短介绍，并完成下述行为模板补全；用户明确要求留空或保留默认值时遵从用户选择。创建前检查是否已有符合用户意图的 Agent，成功后使用服务端生成的 exact `agent_id`，不用显示名猜 locator。

## 创建与补全

1. 根据用户用途确定名称和介绍。头像使用用户指定的有效图片地址或内置 Agent 图标编号；未指定时从内置编号 `1`–`53` 中选择一个并显式传入 `--avatar`。不要把 emoji、角色名称或虚构图片路径当头像标识，也不因用户未指定头像而停下来追问。
2. 创建成功后，读取 [workspaces.md](workspaces.md)，通过 `workspace get --agent-id '<agent-id>' --path 'AGENTS.md'` 获取宿主实际写入的默认模板。介绍只是目录摘要，不能代替行为模板；创建返回成功也不表示模板字段已补全。
3. 基于读到的全文填写 `Role` 中的 `Purpose`、`Responsibilities`、`Out of scope`、`Preferred working style` 等现有字段。结合已知用途写清目标、职责、边界和工作方式；信息不足时保留未知字段，不虚构用户偏好、资质或权限。保留原有标题、字段名、`Baseline Rules` 的全部规则与其他已有内容，不用新写的一段人设替换默认模板，也不复制一份可能过期的模板。
4. `workspace update --content` 是完整覆盖：提交读回原文加字段补全后的完整内容，不能只提交新字段或 Role 段落。补充既有 Agent 的行为模板时同样先读原文，只在相应字段中补充，保留已有字段值；仅用户明确要求改写的内容才替换。
5. 再次 `agent get` 验证头像和介绍，并 `workspace get` 读回 `AGENTS.md`，核对字段已写入、原规则仍完整。若创建已成功但模板补全失败，报告已创建的 exact Agent 与未完成部分，继续针对同一 ID 修复，不重复创建，也不宣称全部完成。

Agent profile/runtime 的修改不属于当前 `nexusctl agent` surface；使用 `nexus-configuration` inspect 当前 `agents` domain，而不是编造 update/delete 子命令。根级 `AGENTS.md` 行为模板属于 workspace 文件，按上述读取、补全、写回流程处理。
