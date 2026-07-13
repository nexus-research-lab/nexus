# 问题卡片

- `ask-user-question-card-model.ts` 只投影卡片摘要、色调、占位文案和选项视觉状态。
- `ask-user-question-card.tsx` 只持有单卡展开状态并组合头部与正文。
- `ask-user-question-card-header.tsx` 负责问题身份、折叠摘要和选择数量。
- `ask-user-question-card-body.tsx` 负责选项列表与自定义回答输入。

选择互斥规则属于上层问答模型；本目录只发送用户意图，不得直接修改回答草稿。
