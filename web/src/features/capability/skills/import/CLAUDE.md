# Skill Import

- `skill-import-dialog.tsx` 只拥有弹窗生命周期和区块组合，不维护来源表单或静态规范内容。
- `use-skill-import-dialog.ts` 独占 Git 草稿、关闭保护、焦点和提交入口；切换来源时保留草稿，关闭时清理。
- `skill-import-dialog-model.ts` 保存封闭模式表与纯提交规则，不从 JSX 反推业务状态。
- 来源区通过模式到视图的穷举映射选择 Git 或本地导入，不追加条件渲染矩阵。
- 规范下载和 Footer 命令分别归 Guide 与 Footer，视图只消费自己声明的窄接口。
