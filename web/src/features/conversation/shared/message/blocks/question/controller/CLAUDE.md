# 用户问答控制层

- `use-ask-user-question-controller.ts` 只组合作用域、草稿、展开状态和提交能力。
- `question-controller-model.ts` 负责交互状态、初始草稿、草稿变更与提交资格的纯投影。
- `use-question-draft.ts` 负责服务端回答恢复、作用域重置和用户编辑意图。
- `use-question-submission.ts` 负责提交互斥、作用域令牌和成功后的折叠事务。

异步结果只能写回发起时的工具作用域；旧提交完成时不得清除新作用域的提交状态。
