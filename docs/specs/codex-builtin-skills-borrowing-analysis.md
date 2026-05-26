# Codex 内置 Skill 借鉴分析

## 1. 范围与源码版本

本分析只覆盖 Codex 开源代码中作为系统内置预装的 skill，不覆盖 `openai/skills` 仓库的 curated / experimental 外部技能市场。

本次核对的源码锚点：

- `openai/codex`：`464ab40dfa1fd5058ea52512c29f38d2e4f6b204`，提交时间 `2026-05-22`，提交信息 `feat: best-effort compact large tool schemas (#23904)`
- 内置 skill 目录：`codex-rs/skills/src/assets/samples/`
- `openai/skills`：`b0401f07213a66414d84a65cb50c1d226f99485a`，提交时间 `2026-05-21`，`.system` 目录中的 skill 名称与 Codex 源码一致，但部分内容落后于 Codex 源码

结论以 `openai/codex` 源码为准。它当前内置 5 个 skill：

1. `imagegen`
2. `openai-docs`
3. `skill-installer`
4. `skill-creator`
5. `plugin-creator`

## 2. 总体结论

| Skill | 是否建议借鉴 | 建议方式 | 优先级 |
|---|---:|---|---:|
| `skill-creator` | 是 | 改造成 Nexus skill 创建器与校验器 | P0 |
| `skill-installer` | 是 | 改造成 Nexus skill 导入 / 安装助手 | P0 |
| `imagegen` | 是，已有基础 | 同步透明图、prompt 分类、后处理校验等增量规则 | P1 |
| `openai-docs` | 条件建议 | 作为开发者/Provider 文档能力，不默认托管给所有 agent | P1 |
| `plugin-creator` | 暂不直接引入 | 借鉴 manifest-first scaffold / validate 模式，等 Nexus 有插件包体系后再落地 | P2 |

最高价值不是直接拷贝五个目录，而是借鉴 Codex 的三层设计：

- **系统内置 skill 可版本化预装**：Codex 把内置 skill 编进二进制，启动时写到 `$CODEX_HOME/skills/.system`，用 marker fingerprint 避免重复覆盖。
- **progressive disclosure**：模型默认只看到 name / description / path；命中后再读 `SKILL.md`，需要时才读 `references/` 或执行 `scripts/`。
- **产品元数据与执行说明分离**：`agents/openai.yaml` 承载 UI、默认 prompt、工具依赖和 policy；`SKILL.md` 保持给 Agent 的执行规则。

## 3. Codex Skill 机制中值得借鉴的部分

### 3.1 内置目录与缓存机制

Codex 在 `codex-rs/skills/src/lib.rs` 中用 `include_dir!` 内嵌系统 skill，启动时安装到：

```text
$CODEX_HOME/skills/.system
```

并写入 `.codex-system-skills.marker`。marker 由目录结构和文件内容 hash 得出，匹配时跳过重装。

Nexus 当前做法是 workspace 初始化时把系统 skill 部署到：

```text
<workspace>/.agents/skills/<skill_name>/
<workspace>/.claude/skills/<skill_name> -> ../../.agents/skills/<skill_name>
```

这符合 Nexus 的“运行时消费 workspace 副本”的原则。可借鉴的是 **版本指纹**，不是目录形态。建议：

- 给 `skills/<name>/` 计算内容 fingerprint。
- 部署到 workspace 时记录 `.nexus-system-skill.marker` 或统一 manifest。
- fingerprint 未变时跳过覆盖，减少初始化副作用。
- fingerprint 变更时只更新系统托管 skill，继续避免覆盖用户 workspace 文件。

### 3.2 Scope 与加载优先级

Codex 有四类 scope：

- `repo`
- `user`
- `system`
- `admin`

加载优先级是 `repo > user > system > admin`，并支持按 `path` 或 `name` 禁用。Nexus 当前有：

- system skill：`imagegen`、`memory-manager`、`scheduled-task-manager`
- internal skill：`nexus-manager`
- builtin / external / workspace skill
- scope：`any`、`main`、`room`

Nexus 不需要照搬 Codex scope，但可以补两个能力：

- 对系统托管 skill 增加启停策略，而不是只靠 hard-coded `systemSkillNames`。
- 对同名 skill 冲突给出确定优先级和 UI 提示，避免外部导入覆盖系统语义。

### 3.3 `agents/openai.yaml` 元数据

Codex 的 `SKILL.md` frontmatter 只要求 `name` 和 `description`。UI 信息、依赖和 policy 放到 `agents/openai.yaml`：

```yaml
interface:
  display_name: "OpenAI Docs"
  short_description: "Reference docs, choose models, and migrate OpenAI API integrations"
  default_prompt: "Use OpenAI Docs for official docs lookup..."

dependencies:
  tools:
    - type: "mcp"
      value: "openaiDeveloperDocs"
      transport: "streamable_http"
      url: "https://developers.openai.com/mcp"

policy:
  allow_implicit_invocation: true
```

Nexus 当前 frontmatter 已经扩展了 `title`、`scope`、`tags`、`category_key` 等字段，适合现阶段目录展示；但它把产品展示字段和 agent 执行触发字段混在一起。建议新增兼容层：

- 短期：继续支持现有 frontmatter，不破坏已导入 skill。
- 中期：支持 `agents/nexus.yaml`，字段覆盖 UI 展示、默认 prompt、依赖声明、policy。
- 长期：把 marketplace / UI 信息从 `SKILL.md` 中迁出，减少模型上下文噪音。

### 3.4 MCP 依赖声明

`openai-docs` 通过 `agents/openai.yaml` 声明 MCP 依赖。Codex 会在 skill 被显式使用时检查缺失 MCP server，并在用户确认后写入全局 config、刷新 MCP server。

Nexus 有自己的 MCP runtime 和 `nexus_automation` 能力。建议借鉴声明模型，但不要直接写全局 Codex config：

- skill manifest 可声明 `dependencies.tools`。
- Nexus 在安装或启用 skill 时做依赖检查。
- 缺失依赖时提示用户安装或绑定 connector / MCP server。
- 对系统内置依赖使用平台托管配置，不交给 skill 文本临时解释。

## 4. 逐项分析

### 4.1 `skill-creator`

**用途**

创建或更新 Codex skill。它定义了 skill 的目录结构、命名规则、progressive disclosure 原则、脚本 / references / assets 的使用边界，并提供：

- `scripts/init_skill.py`
- `scripts/quick_validate.py`
- `scripts/generate_openai_yaml.py`
- `references/openai_yaml.md`

**值得借鉴点**

- 把 skill 当作“给另一个 Agent 的 onboarding guide”，而不是普通用户文档。
- 强调 `description` 是触发入口，必须写清楚何时使用。
- 强调 `SKILL.md` 保持精简，复杂细节拆到 `references/`。
- 对脆弱流程使用 `scripts/` 固化，避免 Agent 每次重写。
- 提供 validator，避免无效 frontmatter、命名不合规、缺必填字段。
- 引导创建 `agents/openai.yaml` 这类产品元数据。

**Nexus 适配**

建议做成 `skill-creator` 或 `nexus-skill-creator`，但不要原样使用 Codex 的脚本。Nexus 需要校验自己的格式：

- frontmatter：`name`、`title`、`description`、`scope`、`tags`、`category_key`、`category_name`、`recommendation`、`version`
- scope：`any`、`main`、`room`
- 部署形态：`.agents/skills/<skill_name>/` 与 `.claude/skills/<skill_name>` symlink
- 外部导入 manifest：`.nexus-skill.json`
- 可选元数据：未来的 `agents/nexus.yaml`

**建议落地**

P0。先落一个 Nexus 版脚手架和校验器：

```text
skills/skill-creator/
  SKILL.md
  scripts/init_nexus_skill.py
  scripts/validate_nexus_skill.py
  references/nexus_skill_format.md
```

这个 skill 应该优先给主智能体或开发者 agent 使用，不必自动托管给所有普通 agent。

### 4.2 `skill-installer`

**用途**

从 `openai/skills` curated / experimental 列表或任意 GitHub repo 路径安装 skill 到 `$CODEX_HOME/skills`。核心脚本：

- `scripts/list-skills.py`
- `scripts/install-skill-from-github.py`
- `scripts/github_utils.py`

安装脚本支持：

- GitHub URL 或 `owner/repo + path`
- public repo 直接下载 zip
- 私有 repo / 权限失败时 git sparse checkout fallback
- 安装前检查 `SKILL.md`
- 目标目录已存在则中止
- 多 path 批量安装

**值得借鉴点**

- “列出可安装 skill”和“从 GitHub 路径安装”分开。
- 下载优先、git sparse checkout 兜底，兼顾速度和私有仓库。
- 防 zip path traversal。
- 目标已存在时默认不覆盖。
- 列表输出标记 already installed。
- 明确区分 curated、experimental、system。

**Nexus 适配**

Nexus 已经有更完整的二阶段模型：

```text
import -> registry -> install to agent workspace
```

因此不应照搬 `$CODEX_HOME/skills` 安装语义。应改造成：

- `nexusctl skill search` / `nexusctl skill import-git` / `nexusctl skill import-skills-sh`
- 导入到 Nexus registry，而不是写当前 agent workspace。
- 安装时再选择 agent。
- 对 `skills/.system` 提示“系统托管，不建议手动导入”。
- 对 GitHub path import 增加 Codex 的安全检查与 sparse checkout fallback。

**建议落地**

P0。它直接补强 Nexus skill 生态的入口。建议做成主智能体可用的管理 skill，或者合并进 `nexus-manager` 的 skill 管理章节，并把确定性逻辑下沉到 Go CLI。

### 4.3 `imagegen`

**用途**

生成或编辑位图资产。Codex 版本有两条路径：

- 默认：内置 `image_gen` tool
- fallback：`scripts/image_gen.py`，显式 CLI / API / model 控制时使用

它还带有透明图策略：默认先生成纯色 chroma-key 背景，再用 `scripts/remove_chroma_key.py` 本地去背；只有复杂透明需求才确认后切到 `gpt-image-1.5` 原生透明 fallback。

**值得借鉴点**

- 明确区分 bitmap 与 SVG / HTML / Canvas，避免把所有视觉需求都推给图片生成。
- 区分 generate / edit / reference image / edit target。
- 多资产用多次调用，不把多个不同资产塞进一个 prompt。
- 对透明图有可验证的本地后处理流程。
- prompt taxonomy 很完整，能帮助 Agent 选择合适提示词形态。
- 对项目资产有保存路径规则：项目引用的资产不能只留在 Codex 默认输出目录。

**Nexus 现状**

Nexus 已有 `skills/imagegen`，并且已经做了更符合本平台的改造：

- 唯一执行入口是 `nexusctl imagegen`。
- Provider、鉴权、接口兼容和响应解析由 Go 服务负责。
- 普通单图走快路径，避免读取 reference、避免反复打开二进制图片。
- 输出以 CLI JSON 的 `item.path` / `item.markdown` 为真相源。

**建议落地**

P1，不建议替换 Nexus 版本，只做增量同步：

- 同步 Codex 最新透明图流程与 `remove_chroma_key.py` 的参数说明。
- 增补 generate/edit taxonomy，用于复杂任务，不影响普通快路径。
- 检查 Nexus `references/prompting.md` 是否覆盖 `text-localization`、`identity-preserve`、`compositing`、`scientific-educational` 等分类。
- 保持 Nexus 的低轮次快路径，避免 Codex 版本过重。

### 4.4 `openai-docs`

**用途**

查询 OpenAI 官方开发文档、选择最新模型、做模型迁移和 prompt upgrade。它优先使用 `developers.openai.com` MCP server，并通过 `agents/openai.yaml` 声明依赖：

```text
openaiDeveloperDocs -> https://developers.openai.com/mcp
```

还提供：

- `references/latest-model.md`
- `references/upgrade-guide.md`
- `references/prompting-guide.md`
- `scripts/resolve-latest-model-info.js`

**值得借鉴点**

- 对高变化信息强制使用官方文档，不依赖模型记忆。
- 明确 fallback 顺序：MCP -> 官方域名搜索 -> bundled fallback，并要求披露 fallback。
- 把“最新模型”当成动态事实，用脚本从文档中的 metadata 提取目标模型和迁移链接。
- 限制模型升级范围：只改模型默认值和直接相关 prompt，不顺手迁移 SDK / provider / 环境变量。
- 在 skill 元数据里声明外部工具依赖。

**Nexus 适配**

这个 skill 对 Nexus 有价值，但不适合作为所有 agent 的系统托管 skill：

- 它强依赖 OpenAI 官方文档和 MCP。
- Nexus 可能支持多个 Provider，不能把 OpenAI 视为唯一默认。
- 它更适合“开发者 agent / provider 集成 agent / 模型配置诊断”场景。

**建议落地**

P1。建议改造成 `provider-docs` 或保留 `openai-docs` 作为可选 builtin：

- OpenAI 专属部分保留官方域名约束。
- Nexus Provider 文档可扩展成 `references/openai.md`、`references/anthropic.md`、`references/gemini.md` 等。
- 动态文档查询通过 Nexus connector / MCP 依赖声明管理。
- 不要把 bundled `latest-model.md` 当长期真相源，只能作为断网 fallback。

### 4.5 `plugin-creator`

**用途**

创建 Codex plugin 目录，生成 `.codex-plugin/plugin.json`，可选生成 skills / hooks / scripts / assets / MCP / app 配置，并维护个人 marketplace entry。核心脚本：

- `scripts/create_basic_plugin.py`
- `scripts/validate_plugin.py`
- `scripts/update_plugin_cachebuster.py`
- `scripts/read_marketplace_name.py`

**值得借鉴点**

- manifest-first：先保证插件 manifest 可被平台摄取。
- scaffold 与 validator 配套，避免生成半成品。
- marketplace entry 由工具生成，不让 Agent 手改复杂 JSON。
- 本地开发用 cachebuster 触发重新安装，而不是反复递增语义版本。
- 对默认个人 marketplace 和 repo/team marketplace 有清晰边界。

**Nexus 适配**

Nexus 当前有 skill、connector、automation、workspace、channel 等能力，但还没有与 Codex plugin 完全对应的“插件包 + marketplace + app/MCP/skill 聚合”模型。直接引入 `plugin-creator` 会制造概念债：

- `.codex-plugin/plugin.json` 与 Nexus 当前数据模型不匹配。
- marketplace policy 字段不等于 Nexus skill catalog。
- Codex deeplink / reinstall flow 对 Nexus 无意义。

**建议落地**

P2。暂不引入为用户可见 skill。先把它当成未来能力包设计参考：

- 如果 Nexus 要做“能力包”，可定义 `.nexus-plugin/plugin.json` 或 `.nexus-capability/manifest.json`。
- 借鉴 scaffold + validate + marketplace entry 生成模式。
- 先服务 connector / MCP / skill 聚合包，再考虑 UI deeplink 和本地 reinstall flow。

## 5. 推荐路线图

### P0：先补生态自举能力

1. 新增 Nexus 版 `skill-creator`。
2. 新增或增强 `skill-installer`，让主智能体能从 GitHub / skills.sh 安全导入 skill。
3. 给 skill 导入链路补 validator，避免无效 skill 进入 registry。

### P1：增强已有运行时质量

1. 同步 `imagegen` 的透明图与复杂 prompt taxonomy。
2. 增加 `agents/nexus.yaml` 或兼容 `agents/openai.yaml` 的元数据读取。
3. 为 skill 增加依赖声明，先覆盖 MCP / connector。
4. 将 `openai-docs` 做成可选开发者 skill，而不是默认托管 skill。

### P2：能力包 / 插件化

1. 设计 Nexus capability package manifest。
2. 借鉴 `plugin-creator` 的 scaffold、validate、cachebuster、marketplace entry 模式。
3. 等 Nexus 有对应安装 / 启用 / UI 呈现链路后，再落用户可见 skill。

## 6. 不建议直接照搬的部分

- 不要把 Codex 的 `$CODEX_HOME/skills` 作为 Nexus runtime 真相源。Nexus 的真相源仍应是 workspace 下 `.agents/skills`。
- 不要把 `agents/openai.yaml` 字段原样变成 Nexus 唯一标准。可以兼容读取，但 Nexus 应定义自己的 `agents/nexus.yaml`。
- 不要把 `plugin-creator` 直接暴露给用户。Nexus 目前还没有 Codex plugin marketplace 等价物。
- 不要让 `openai-docs` 影响非 OpenAI Provider 的默认推荐。它应是 OpenAI 官方文档入口，而不是通用模型事实源。
- 不要把 `imagegen` 的 fallback CLI 替换 Nexus Go 服务。Nexus 已经把 provider 和鉴权放在后端，这是更适合产品化的边界。

## 7. 最小落地清单

如果只选最少动作，建议先做这 4 件：

1. `skills/skill-creator`：Nexus skill scaffold + validate。
2. `nexusctl skill validate <path>`：Go 侧校验 frontmatter、目录结构、scope、危险文件。
3. `nexusctl skill import-git` 增强：参考 Codex installer 的 zip 安全解压、sparse checkout fallback、目标存在保护。
4. `skills/imagegen` 增量同步透明图规则：保留快路径，只在复杂任务读取 reference。

这样能先提升 Nexus skill 生态的生产质量，同时避免过早引入 Codex plugin 概念。
