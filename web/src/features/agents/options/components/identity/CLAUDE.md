# identity/ - Agent 身份视图

- agent-profile-file-editor.tsx 只承载普通 Agent 根级 AGENTS.md 的 Markdown 预览、编辑和确认保存；主 Nexus 没有该文件，身份页不渲染此编辑器，不复用工作区文件头部或文件保存工具栏。

- `agent-options-identity-tab.tsx` 只按布局描述组合资料、标签、模型和简介字段，不维护子域状态。
- `identity-profile-fields.tsx` 负责组合头像、名称与名称校验反馈，并用有序纯规则投影校验提示的语义和色调；同时声明模态编辑的初始焦点目标。没有校验反馈时不得保留占位高度，避免窄屏字段组之间形成伪分区。
- `identity-avatar-picker.tsx` 以放大的当前头像和明确的“更换头像”文字作为唯一入口，点击后通过共享锚定浮层展示五列大图标网格；身份表单不得恢复横向拖拽、翻页按钮或常驻头像长列表，也不得只靠无文字的小图标暗示入口。
- `identity-vibe-tags.tsx` 独占待添加标签草稿并绑定编辑作用域，父级只持有已确认标签集合；标签与输入在窄窗口自然换行，不形成横向滚动轨道。
- `identity-model-selector.tsx` 统一 Provider/模型选项投影及选择值编解码。
- `identity-layout.ts` 用完整描述表维护 dialog/inline 布局差异；dialog 在 `md` 以下堆叠资料、标签与模型字段，并维持可触控的字段高度和分组间距，覆盖手机和窄平板而不复制整套 JSX；inline 窄屏列必须由内容自然撑高，只有桌面横向布局才允许资料列伸展，避免字段组之间出现无语义空白带。
