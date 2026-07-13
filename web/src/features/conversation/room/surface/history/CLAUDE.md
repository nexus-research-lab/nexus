# Room 历史

- `room-history-model.ts` 一次性完成排序、活动项、外部 Session 标签和管理能力投影。
- `room-history-item-model.ts` 将活动态、读取/编辑模式、元信息和可用动作投影为封闭视图模型。
- `room-history-item-view.tsx` 通过内容与动作映射渲染，不从原始会话重新判断权限或协议。
- 外部 Session、主对话和最少保留数量只影响布尔能力，不维护未展示的禁用原因。
- 标题编辑以可空草稿表达完整状态，条目视图不维护第二份 `isEditing`。
- 主 Surface 只装配列表、空状态与删除确认，不解释会话协议。
