# Task Form

- 表单草稿的跨字段不变量只在 `use-task-form.ts` 中维护。
- 初始化和提交都消费明确的 `TaskFormDraft`，不拼装宽松字段袋。
- Room 目标强制使用现有绑定会话；脚本强制使用临时执行且不回传。
- 基础表单由 `task-basics-model.ts` 统一投影目标、会话和文案，`task-basics-advanced.tsx` 只组合窄字段视图。
- 视图只消费自己声明的 model/actions 接口，不从资源对象重新推导业务状态。
