# Room 成员弹窗

## 职责边界

- `create-room-dialog.tsx` 只负责弹窗生命周期、区块组合和提交入口。
- `use-create-room-form.ts` 独占表单状态、不变量归一化和提交模型构造；成员移除后群主失效等联动必须在这里完成。
- `room-settings-form.tsx`、`room-member-selector.tsx`、`room-skills-selector.tsx` 只负责各自视图和用户输入，不在渲染期修正状态。
- `use-room-skill-options.ts` 独占技能选项获取、过滤和加载状态。

## 约定

- 弹窗内容通过 React `key` 按初始值重建，禁止引入渲染期 `setState` 或逐字段重置。
- 创建与管理共用 `RoomDialogSubmission` 对象提交，禁止退回位置参数。
- 创建与管理标签通过静态 key 表投影，纯函数只接受该表覆盖范围内的窄翻译签名。
- 表单规则通过归一化数据结构表达，视图不得复制群主、成员和自动回复之间的约束。
- 管理模式只产生完整提交对象；成员差异和跨接口写入事务归页面命令层，弹窗不得直接调用 Room API。
