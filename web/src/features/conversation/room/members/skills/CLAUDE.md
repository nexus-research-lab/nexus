# Room Skills

- Room 技能选择是成员弹窗的业务控件，不扩张为只有一个消费者的共享 MultiSelect API。
- `use-room-skill-options.ts` 独占技能资源加载、过滤和加载错误；Room 内置 Skill 使用共享双语展示说明参与显示与搜索，视图不直接请求 API。
- `room-skill-multi-select-model.ts` 统一投影互斥菜单状态、已选项和选择变更。
- 业务菜单复用 `shared/ui/menu` 的 `SelectMenuTrigger`、锚点、尺寸、listbox 框架和 `SelectMenuOptionRow`，不得复制触发器/选项 DOM、ARIA 或浏览器定位生命周期；绝对定位触发层与已选 Chip 仍保持原有兄弟结构，多选状态和移除命令留在本域。
- 已选 Chip 需要独立移除热区和自然换行，字段保留内容驱动的最小高度；这只属于多选内容几何，不是第二套单选触发器字号、材质或焦点样式。
- 加载、错误、空态和选项列表互斥；新增状态时扩展规则表和对应视图。
- 加载图标使用共享 Spinner 配方，菜单内错误和表单级读取失败都使用 `UiInlineNotice`；菜单只决定互斥状态，表单只提供标题与影响说明，两者不得复制动效、圆角、颜色或排版。
