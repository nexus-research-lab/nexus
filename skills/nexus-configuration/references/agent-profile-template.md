# 修改已有 Agent 的行为模板

`profile_template` 是创建时初始化 workspace 根级 `AGENTS.md` 的输入。创建后补充、修改或重写角色职责与工作方式，编辑该文件；名称、头像、目录摘要与 runtime 配置使用 `agents` 配置域。

- 修改自己的模板：使用原生文件工具读取、编辑并读回 `AGENTS.md`。
- 修改其他 Agent 的模板：由主智能体读取 `nexus-manager` 的 `references/workspaces.md`，按 exact Agent ID 读取、更新并读回 `AGENTS.md`；普通 Agent 请用户切换到主智能体。

编辑以基础模板为底，填写或按用户要求调整 `Role` 中的 `Purpose`、`Responsibilities`、`Out of scope`、`Preferred working style`，也可按需新增角色字段或专用规则。保留 `Baseline Rules` 标题及全部基础规则，不删除、替换或用新增规则抵消它们；重写角色描述也只作用于相应角色内容。写回完整文件后读回，核对角色内容与基础规则。读回只能证明文件已保存，不能据此宣称活跃 runtime 已加载新规则。
