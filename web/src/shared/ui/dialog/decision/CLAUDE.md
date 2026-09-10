# Decision Dialog

本目录负责需要用户确认或输入值的短决策弹窗，不拥有业务事务和反馈状态。

- `decision-dialog-model.ts` 以描述表定义确认变体，并以有序规则解释 Prompt 键盘提交。
- `decision-dialog-frame.tsx` 统一 Portal、Backdrop、Shell 和操作栏结构。
- `decision-dialog.tsx` 渲染 Confirm 与 Prompt；关闭时卸载 Prompt 内容，初始焦点只交给共享模态协议。
- 默认按钮与多行快捷键说明跟随 i18n，显式业务文案优先。Prompt 的字段名默认关联标题，独立 inputLabel 只在业务语义不同的场景提供；占位文字保持示例职责。
- Prompt 使用 UiField 精确绑定输入 ID、外部校验错误和快捷键说明，不复制错误排版或 ARIA；busy 期间统一禁用输入、确认、取消与关闭，键盘和遮罩也不能绕开。busy 只表达调用方状态，不推断命令受理或成功。
- 输入法事件在调用纯键盘投影之前过滤；单行 Enter、多行 Cmd/Ctrl+Enter 保持原始值提交，多行普通 Enter 保留换行。公共模态/浮层分别过滤自己的全局候选键，不由 Prompt 复制全局事件监听。

消费者必须在外部拥有打开状态、待处理目标和提交命令。Prompt 校验失败时可保持打开，组件不得推断业务成功。
