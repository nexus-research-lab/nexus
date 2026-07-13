# 用户问答块

- `ask-user-question-block.tsx` 只适配工具协议并组合控制器与视图。
- `ask-user-question-model.ts` 校验未知工具输入，并维护原子回答草稿、结果恢复、提交投影与交互状态；视图不得自行断言协议或拼装答案。
- `controller/` 分离草稿恢复、交互状态、展开生命周期和异步提交事务；异步提交只能更新发起时的工具作用域。
- `card/` 分离卡片投影、折叠头部、选项列表和自定义回答输入；卡片只发送用户意图，不维护草稿规则。
- `ask-user-question-timeout.ts` 只解释问答工具结果的超时错误码，协议类型不保存运行规则。
- `ask-user-question-view.tsx` 与 `ask-user-question-header.tsx` 只编排问答列表、提交区和终态提示，交互文案与图标由状态表驱动。
- 单选项与自定义回答互斥，多选项可附加自定义回答；该约束必须在模型转换函数中保持原子更新。
- SDK 的 `multiSelect` 只在输入解析时兼容，内部问题契约统一使用 `multi_select`。
