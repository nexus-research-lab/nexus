# Task Form

- 表单草稿的跨字段不变量只在 `use-task-form.ts` 中维护。
- 初始化和提交都消费明确的 `TaskFormDraft`，不拼装宽松字段袋。
- 提交层分别投影执行位置与真实结果接收 Session；创建来源只作 provenance，不参与二者映射。
- 编辑初始化按当前任务的执行与投递配置投影合法组合；旧裸路由、合成收件箱或已失效绑定清空选择并要求重绑。
- DM 执行与投递按“Agent → DM/active IM Session”选择；Room 按“Room → 共享 Room Session → 该 Session 的成员 Agent”选择。Room 成员可留空并在提交时解析为房主；执行 Agent 与投递回复 Agent 是独立字段。
- 历史脚本任务只读，不进入表单提交。
- 基础表单由 `task-basics-model.ts` 统一投影目标、会话和文案，`task-basics-advanced.tsx` 只组合窄字段视图。
- 执行、投递与失效重绑提示统一使用 `UiPanel` 的语义 padding/radius；字段帮助、提示标题与高级摘要统一使用 Typography role，高级区统一组合 `UiDisclosure`，业务层不得再渲染或装饰原生 `details/summary`。
- 视图只消费自己声明的 model/actions 接口，不从资源对象重新推导业务状态。
