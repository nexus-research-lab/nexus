# 其他 Agent workspace

只在主智能体需要读取或修改另一个 Agent 的 workspace 时读取本文件。当前 Agent 自己的 workspace 直接使用原生文件工具，不绕到 `nexusctl`。

```bash
nexusctl --json workspace list --agent-id '<agent-id>'
nexusctl --json workspace get --agent-id '<agent-id>' --path '<relative-path>'
nexusctl --json workspace create --agent-id '<agent-id>' --path '<relative-path>' --type file --content '<content>'
nexusctl --json workspace create --agent-id '<agent-id>' --path '<relative-path>' --type directory
nexusctl --json workspace update --agent-id '<agent-id>' --path '<relative-path>' --content '<content>'
nexusctl --json workspace rename --agent-id '<agent-id>' --path '<relative-path>' --new-path '<relative-path>'
nexusctl --json workspace delete --agent-id '<agent-id>' --path '<relative-path>'
```

- path/new-path 必须是该 workspace 内的相对路径，不传绝对路径，不使用 `..` 穿越，不访问 state/runtime/database。
- 普通 workspace 文件包括根级 `AGENTS.md`、`MEMORY.md` 与 `memory/` 正文；`.agents`、`.claude` 等内部目录不在本入口的读写范围内。Skill 内容编辑按 [skills.md](skills.md) 分流。
- 修改前先 get 原内容并确认覆盖范围；create 前确认目标不存在；rename 前确认 source/target；delete 前精确核对路径和影响。
- `update --content` 是完整覆盖，不是 patch/append。需要局部修改时先读回、在模型侧形成完整新内容，再一次更新；不要在并发修改不明时覆盖。
- 补充、修改或重写根级 `AGENTS.md` 行为模板时，遵循 [accounts-and-agents.md](accounts-and-agents.md) 的角色编辑规则，在基础模板上调整角色字段或新增专用规则，保留 `Baseline Rules` 标题及全部基础规则。
- 不把密码、token、Connector credential 或其他秘密写入 workspace。大型二进制、附件和 Room 公共产物使用对应 artifact/attachment 流程，不塞进文本 content flag。
