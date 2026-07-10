# Task Form

- 表单草稿的跨字段不变量只在 `use-task-form.ts` 中维护。
- 初始化和提交都消费明确的 `TaskFormDraft`，不拼装宽松字段袋。
- Room 目标强制使用现有绑定会话；脚本强制使用临时执行且不回传。
- 视图只消费自己声明的 model/actions 接口。
