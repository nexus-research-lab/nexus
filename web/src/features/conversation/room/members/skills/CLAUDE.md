# Room Skills

- Room 技能选择是成员弹窗的业务控件，不扩张为只有一个消费者的共享 MultiSelect API。
- `use-room-skill-options.ts` 独占技能资源加载、过滤和加载错误；视图不直接请求 API。
- `room-skill-multi-select-model.ts` 统一投影互斥菜单状态、已选项和选择变更。
- 业务菜单复用 `shared/ui/menu` 的锚点、尺寸和 listbox 框架，不复制浏览器定位生命周期。
- 加载、错误、空态和选项列表互斥；新增状态时扩展规则表和对应视图。
