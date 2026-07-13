# Decision Dialog

本目录负责需要用户确认或输入值的短决策弹窗，不拥有业务事务和反馈状态。

- `decision-dialog-model.ts` 以描述表定义确认变体，并以有序规则解释 Prompt 键盘提交。
- `decision-dialog-frame.tsx` 统一 Portal、Backdrop、Shell 和操作栏结构。
- `decision-dialog.tsx` 渲染 Confirm 与 Prompt；关闭时卸载 Prompt 内容，初始焦点只交给共享模态协议。

消费者必须在外部拥有打开状态、待处理目标和提交命令。Prompt 校验失败时可保持打开，组件不得推断业务成功。
