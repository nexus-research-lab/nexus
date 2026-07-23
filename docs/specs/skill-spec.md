# Skill 规范

## 1. 文档目标

本文档定义 Skill 的来源、运行时发现路径、Agent 选择状态和更新边界。当前平台同时支持 nxs 与 Claude Code，因此“源文件”和“Agent 是否启用”必须分开建模。

## 2. Skill 的几层形态

### 2.1 平台源目录

随 Nexus 产品发布的 Skill 位于产品根目录的 `skills/<skill_id>/`。外部 Skill 先进入当前用户的本地 registry，再按 Agent 安装。

### 2.2 平台全局兼容根

平台 Skill 由 `EnsurePlatformSkillLibrary` 同步到全局配置目录：

```
<config>/platform-skills/
  .agents/skills/<skill_id>/SKILL.md
  .claude/skills -> ../.agents/skills
```

`.agents/skills` 是 nxs 的发现入口，`.claude/skills` 是 Claude Code 的发现入口；通常用相对 symlink 指向同一份平台文件。Windows 无目录 symlink 权限时只在这个全局兼容根内生成镜像，不会扩散到 Agent workspace。同步使用源内容指纹和原子替换，产品升级后下一次运行时装配会刷新全局库。

### 2.3 Agent 选择状态

Agent 的 runtime 记录只保存平台 Skill 的稳定 ID：`runtimes.skill_ids_json`，对外投影为 `options.skill_ids`。当前内置 catalog 的 ID 与 Skill canonical name 相同（例如 `ima-skill`），但调用方不应把路径当作 ID。

启动 nxs 或 Claude Code 时，宿主把 Agent 的 ID 列表转换为 SDK 的 Skill 选择器，并把平台兼容根作为显式 additional directory 传入。nxs 在目录发现阶段按名称过滤；Claude Code 通过 stream-json initialize 的 `skills` 字段过滤主会话上下文，并额外生成 `Skill(<id>)` 权限规则以兼容 CLI 权限模型。Agent workspace 不再保存平台 Skill 副本。

### 2.4 外部与 workspace-local Skill

外部 Skill 仍按 owner registry 管理，安装后部署到：

- `<workspace>/.agents/skills/<skill_id>/`
- `<workspace>/.claude/skills/<skill_id>`（兼容入口）

workspace-local Skill 只属于当前 workspace，不进入平台全局库。迁移期间仍兼容读取旧 workspace 文件。

目录来源与启用策略是两条独立维度。catalog 保留 `source_type=builtin` 的兼容值，
并通过 `source_kind` 区分：`nexus_platform` 表示产品随包并同步到平台全局库，
`user_global` 表示用户的 `~/.codex/skills`、`~/.agents/skills` 或
`~/.cc-switch/skills`。前者可以按 Agent ID 直接选择，后者仍按用户显式导入/部署；
二者都不能被解释为“默认启用”。

### 2.5 Room Skill

`scope: room` 不部署到单个 Agent workspace。Room runtime 根据 Room 配置读取 Skill 的 `ReadmeMarkdown`，去掉 frontmatter 后直接注入成员运行时；Skill 正文是 Room 规则的唯一真相源。

## 3. 当前真相源

### 3.1 平台 Skill

- 文件真相源：产品 `skills/` 经同步生成的全局兼容根
- Agent 启用真相源：`runtimes.skill_ids_json`
- catalog 与 UI：上述两者的投影，不自行复制文件

### 3.2 外部 Skill

- 源文件真相源：owner registry
- Agent 安装真相源：workspace 部署文件（兼容旧版）
- catalog 与 UI：registry manifest 和 workspace 状态的投影

### 3.3 Room Skill

Room 配置选择的 Skill 正文由 Room runtime 直接读取并注入，不复用 Agent 的 `skill_ids`。

## 4. Skill 分类

### 4.1 public skill

- 对外可见
- 可在能力页 / marketplace 中展示

### 4.2 internal skill

- 系统内部使用
- 不进入普通公开 marketplace

### 4.3 platform managed skill

- 随产品发布并由平台统一同步
- Agent 只保存 ID，不能通过 workspace 文件修改其内容

## 5. 生命周期

### 5.1 发布与同步

产品包更新 `skills/` 后，平台库同步器计算源指纹；指纹变化时构建临时兼容根并原子替换 `<config>/platform-skills`。同步失败必须阻止对应 runtime 启动，避免使用半份 Skill 库。

### 5.2 安装

安装平台 Skill 只向 Agent 的 `skill_ids` 追加 ID；安装外部 Skill 才会产生 workspace 部署副本。

### 5.3 卸载

卸载平台 Skill 只从 Agent 的 `skill_ids` 移除 ID，不删除全局源文件。卸载外部 Skill 清理当前 Agent 的 workspace 副本，不破坏 registry 源。

### 5.4 运行时装配

宿主统一把 `skill_ids` 传给 bridge：nxs 使用 `WithSkills` 过滤目录发现，Claude Code 使用 initialize `skills` 字段过滤主会话可见 Skill，并用 `Skill(<id>)` allow rule 处理 CLI 权限兼容；两个 runtime 都接收同一个平台兼容根。

## 6. 当前约束

- 平台 Skill 不能在每个 Agent workspace 复制一份
- 不允许把文件路径或 workspace 路径当作平台 Skill ID
- `.claude/skills` 只作为发现入口，不承载另一份平台源
- 外部 Skill 与 workspace-local Skill 可以保留 workspace 文件模型
- Room Skill 由 Room 配置选择，不写入 Agent `skill_ids`

## 7. 禁止项

- 直接修改 `<config>/platform-skills` 中的平台 Skill 文件
- 只改数据库或只改文件而不保持平台 ID 与源库一致
- 把平台 Skill 安装成 owner registry 的外部副本
- 把 internal Skill 混进 public marketplace

## 8. 一句话总结

平台 Skill 是“全局一份源文件 + Agent 记录 ID + runtime 显式选择”；外部 Skill 仍是“registry 源文件 + workspace 部署副本”，Room Skill 则由 Room runtime 直接注入正文。
