# 用户问答块

- `ask-user-question-block.tsx` 只适配工具协议并组合控制器与视图。
- `pending-human-question.tsx` 把稳定 `request_id` 的 pending interaction 适配为可回答或拒绝的结构化输入面，供 DM/Room Composer 替换面共同消费。
- `ask-user-question-model.ts` 校验未知工具输入，并维护原子回答草稿、结果恢复、提交投影与交互状态；视图不得自行断言协议或拼装答案。
- `controller/` 分离草稿恢复、交互状态、展开生命周期和异步提交事务；异步提交只能更新发起时的工具作用域。
- `ask-user-question-item.tsx` 以原生 fieldset/input 语义渲染单个问题、行式选项和内联自定义回答；只发送用户意图，不维护草稿规则。
- `ask-user-question-timeout.ts` 只解释问答工具结果的超时错误码，协议类型不保存运行规则。
- `ask-user-question.css` 只定义行级中性选中态、焦点边界和克制动效，不恢复选项卡片、局部阴影或多层边框。
- `ask-user-question-view.tsx` 只编排问答列表、与权限确认一致的拒绝/提交决策行和终态摘要，交互文案与图标由状态表驱动。
- 选项树只允许由 `pending-human-question.tsx` 经 Composer replacement surface 挂载；消息正文、Room Thread、历史结果和过程展开只显示静态工具证据，不得直接消费 `ask-user-question-block.tsx`。
- 单选项与自定义回答互斥，多选项可附加自定义回答；该约束必须在模型转换函数中保持原子更新。
- Composer 已是问答的唯一外层 task surface；问题、选项、自定义回答和 footer 禁止再次套独立卡片。
- SDK 的 `multiSelect` 只在输入解析时兼容，内部问题契约统一使用 `multi_select`。
- 问答提交属于标准动作，使用共享 `md` Spinner；视图不得自行维护旋转、尺寸或 reduced-motion class。
