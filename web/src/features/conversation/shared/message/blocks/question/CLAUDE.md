# 用户问答块

- `ask-user-question-block.tsx` 只适配工具协议并组合控制器与视图。
- `ask-user-question-model.ts` 维护原子回答草稿、结果恢复、提交投影与交互状态；视图不得自行拼装答案。
- `use-ask-user-question-controller.ts` 负责工具作用域重置、终态同步和提交互斥；异步提交只能更新发起时的工具作用域。
- `ask-user-question-view.tsx` 与 header/card 只负责展示，交互状态的文案和图标由状态表驱动。
- 单选项与自定义回答互斥，多选项可附加自定义回答；该约束必须在模型转换函数中保持原子更新。
