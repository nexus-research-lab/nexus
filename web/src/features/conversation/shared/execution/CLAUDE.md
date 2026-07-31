# Execution WorkGraph

- `use-execution-resource.ts` 只读取后端 `ExecutionView`，活动态轮询是断线恢复，不派生第二套状态机。
- `execution-process-panel.tsx` 是 DM/Room 共用的唯一 WorkGraph 展示；普通聊天没有 Execution 时继续使用 legacy Todo 进程。
- `execution-process-model.ts` 只做标签、色调、计数与依赖展示投影，不猜测 Assignment、Attempt、Submission 或 Acceptance。
- UI 不提供状态写入按钮；编排 mutation 仍由模型语义工具和后端 authority fence 负责。
