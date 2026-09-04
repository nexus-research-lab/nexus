# Skill Import

- `skill-import-dialog.tsx` 只拥有 plain 弹窗生命周期和单列区块组合，不维护来源表单或静态规范内容。
- `use-skill-import-dialog.ts` 独占 Git 草稿、关闭保护、焦点和提交入口；切换来源时保留草稿，关闭时清理。
- `skill-import-dialog-model.ts` 保存封闭模式表与纯提交规则，不从 JSX 反推业务状态。
- 来源区通过模式到视图的穷举映射选择 Git 或本地导入，不追加条件渲染矩阵。
- 规范下载和 Footer 命令分别归 Guide 与 Footer，视图只消费自己声明的窄接口。
- 导入说明、示例 frontmatter 与可下载的 Room 编写指南必须跟随当前语言；导入协议字段和用户文件保持原样。
- 标题区只显示“导入 Skill”；来源模式使用文字分段，格式规则、frontmatter 示例和指南下载默认收在共享 `UiDisclosure` 的次级区域中，不占据导入首屏。
- 来源分段、表单、zip 空投区、示例面板、文字层级与加载图标必须复用 shared/ui；不得在导入子组件重写按钮、字号、圆角或旋转动画。
- 导入是单列紧凑表单，固定使用 `UiDialogFormShell size="md"`；不得因示例内容较长而扩大整窗，溢出交给共享自适应滚动合同。
