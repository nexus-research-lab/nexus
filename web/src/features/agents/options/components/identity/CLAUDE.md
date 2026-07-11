# identity/ - Agent 身份视图

- `agent-options-identity-tab.tsx` 只按布局描述组合资料、标签、模型和简介字段，不维护子域状态。
- `identity-profile-fields.tsx` 负责头像、名称与名称校验反馈。
- `identity-vibe-tags.tsx` 独占待添加标签草稿并绑定编辑作用域，父级只持有已确认标签集合。
- `identity-model-selector.tsx` 统一 Provider/模型选项投影及选择值编解码。
- `identity-layout.ts` 用完整描述表维护 dialog/inline 布局差异，不复制整套 JSX。
