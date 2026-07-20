# identity/ - Agent 身份视图

- agent-profile-file-editor.tsx 只承载根级 AGENTS.md 的 Markdown 预览、编辑和确认保存，不复用工作区文件头部或文件保存工具栏。

- `agent-options-identity-tab.tsx` 只按布局描述组合资料、标签、模型和简介字段，不维护子域状态。
- `identity-profile-fields.tsx` 负责头像、名称与名称校验反馈，并用有序纯规则投影校验提示的语义和色调；同时声明模态编辑的初始焦点目标。
- `identity-vibe-tags.tsx` 独占待添加标签草稿并绑定编辑作用域，父级只持有已确认标签集合。
- `identity-model-selector.tsx` 统一 Provider/模型选项投影及选择值编解码。
- `identity-layout.ts` 用完整描述表维护 dialog/inline 布局差异，不复制整套 JSX。
