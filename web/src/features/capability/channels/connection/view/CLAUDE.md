# Channel Connection View

- `channel-connect-dialog.tsx` 只负责编排弹窗、表单提交和删除确认。
- 字段区与 Footer 各自定义所需的窄控制器接口，不依赖生产者的完整返回类型。
- 展示文案与状态标签归纯模型，视图不得复制状态判断顺序。
