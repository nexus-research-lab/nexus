# 本地 Skill 内容编辑

修改 Skill 的正文、步骤或脚本时，先从当前目录项确认来源与所属 Agent，再选择入口：

- 当前 Agent 自建的 workspace Skill：使用原生文件工具编辑实际目录中的 `SKILL.md` 及相关文件，通常位于 `.agents/skills/<name>/`。连续修改沿用同一目录和名称。
- 其他 Agent 的 workspace Skill：请用户切换到所属 Agent 的对话，由它使用原生文件工具编辑。主智能体的 `nexusctl workspace get/update` 不接受 `.agents`、`.claude` 内部目录，当前没有代改这些文件的专用入口。
- 外部来源更新：使用 `skills` 配置域或 `nexus-manager` 的来源更新操作。平台托管文件由产品维护；这两类内容不按自建 workspace Skill 直接编辑。

同一对话中的旧正文也是快照。编辑前读取当前文件，按本次要求局部修改，保留无关内容；写后读回并检查涉及的引用或脚本。文件写入成功不等于运行中的模型已经加载新内容。

Agent 的启用或停用选择仍使用 `skills` 域的 scoped install/uninstall 操作；内容编辑后需要调整选择时重新 inspect，使用当前 source identity。
