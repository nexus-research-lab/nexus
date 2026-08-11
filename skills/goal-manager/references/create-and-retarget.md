# 创建与纠正 Goal

只在创建新 Goal 或用户明确替换当前 Goal objective 时读取本文件。

## 创建条件

显式 Goal 意图是创建判断的必要条件，但不是立即调用 `create_goal` 的充分条件。普通问题、一次性任务、自动标题和常规协作不能推断为 Goal。

自适应持久化由受管 Execution 的 `promote_execution_to_goal` 完成；只有当前 context 明确开放该动作时才使用。复杂度、Plan 长度、Room 或 Subagent 参与本身都不是持久化证据。

## 信息充分性

创建前确认能够写出完整、具体、可直接执行的 objective。只检查当前任务真正需要的内容：

- 交付物；
- 会改变结果的范围、对象或受众；
- 关键约束；
- 可以判断完成的标准。

能从对话、文件或可靠上下文确定的信息不要重复询问。若缺失信息会实质改变交付物或执行路线，提出最少必要问题并等待；信息足够前禁止调用 `create_goal`，也不要先创建宽泛 Goal 再靠 retarget 补齐。

“写一篇约 1000 字作文”缺少主题、用途或文体时不能创建；“为高二语文作业写约 1000 字议论文，讨论人工智能是否削弱独立思考，立场明确并包含两个论据”已经可以形成 objective。例子只说明信息边界，不是固定模板。

## 创建流程

1. 形成 execution-ready objective。
2. 调用 `get_goal` 确认当前会话没有未结束 Goal。
3. 调用 `create_goal`；只有用户明确给出预算时才传 `token_budget`。
4. 如果当前已经有 objective/owner/scope 兼容的 transient WorkGraph，`create_goal` 会把该 Execution 绑定到新 Goal，不得再创建第二张图。只有两者都尚不存在、且还要建立 WorkGraph 时，才等待 `create_goal` 成功后调用 `prepare_plan_execution`；在这个可选绑定分支中两步有因果顺序，不得放进同一个并行工具批次。Plan 会从 exact active Goal 继承 objective。
5. 创建成功后用一句话确认目标，然后继续执行，不解释底层工具。

第 4 步是可选分支，不是 Goal 创建的隐藏后续。Goal-only 直接继续 Agent Loop；reserved Execution ID 不代表 WorkGraph 已存在；已有兼容 transient WorkGraph 的自动绑定也不代表需要重新 prepare。

## 纠正 objective

只有用户明确表示原 Goal 说错、需要替换为另一个 objective 时才 retarget。普通范围补充、执行建议、当前路线变化或模型自行判断不能触发。

1. 调用 `get_goal` 确认 active Goal。
2. 调用 `retarget_goal`，传入完整替代 objective。
3. 保留同一 Goal 身份和累计用量，不先完成旧 Goal，也不创建新 Goal。
4. Goal-only retarget 应直接返回新 objective revision，仍保持 Goal-only。只有 context 明确证明当前 Goal 已绑定 managed WorkGraph，且工具返回 pending Goal/Execution rebase 时，才按原 target 重试，或先按返回的 `prepare_plan_execution` 指引提交完整 successor Plan Document，再将其原样返回的 `proposal_id` 与 `proposal_digest` 交给 `plan_execution`。MCP retarget 在当前 round 继续完成这两步；REST 或 app-server retarget 由后端启动专用 successor-planning continuation，用户无需再发一条消息。不得自行拼接或复用旧 WorkGraph；Goal-only 若被要求新建 successor Plan，应视为链路错误，不通过造图绕过。

暂停、blocked 或 usage-limited Goal 的显式 retarget 由后端决定如何激活；budget-limited Goal 仍需要用户或控制面调整预算。
