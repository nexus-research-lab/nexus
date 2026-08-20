# Skill 资源

只在主智能体查询 Skill 目录、为 Agent 安装/卸载，或导入/更新外部 Skill 时读取本文件。Agent 自己的配置化 Skill 绑定优先使用 `nexus-configuration`；这里处理 owner 控制面的 Skill 资源。

## 查询与 Agent 绑定

```bash
nexusctl --json skill list --agent-id '<agent-id>' --query '<text>'
nexusctl --json skill get '<skill-name>' --agent-id '<agent-id>'
nexusctl --json skill agent-list --agent-id '<agent-id>'
nexusctl --json skill install --agent-id '<agent-id>' --skill-name '<skill-name>'
nexusctl --json skill uninstall --agent-id '<agent-id>' --skill-name '<skill-name>'
```

install 在 Agent workspace 中可以省略 agent ID 让宿主推断，但主智能体跨 Agent 管理时显式传 exact ID。基础托管 Skill 不手工卸载；卸载前核对名称、来源与依赖。

## 导入与更新

```bash
nexusctl --json skill import-local --path '<local-skill-directory>'
nexusctl --json skill search-external '<query>'
nexusctl --json skill import-git --url '<https-git-url>' --branch '<branch>' --path '<skill-subpath>'
nexusctl --json skill import-external --item-file '<search-item.json>'
nexusctl --json skill install-external --agent-id '<agent-id>' --item-file '<search-item.json>'
nexusctl --json skill update '<skill-name>'
nexusctl --json skill update --all
```

- 外部来源通常先 search，再把返回的 exact item 作为 `--item-file` 交给 import/install；不要从标题手工重建 package/git/raw URL 字段。`--item-json` 与 `--item-file` 互斥。
- import-git 只接受 HTTPS Git URL；branch/path 只在用户或搜索结果明确时传。import-local 必须是受信本地 Skill 目录。
- `update --all` 与 skill name 互斥。更新前先 get/list 当前来源；更新后重新 get，确认版本、状态与 Agent 可见性。
- 外部导入会产生网络和代码供应链影响。只在用户明确要求对应来源时执行，不把搜索结果当安装授权。
