# 图标选择器

- `icon-picker-model.ts` 统一约束列数、布局、尺寸、资源路径和选择态样式。
- `icon-picker.tsx` 只渲染模型给出的清除动作与图标项，不重复判断禁用点击或拼接资源路径。
- 图标族由 `lib/avatar` 定义，本目录不得维护业务侧 Agent 或 Room 图标范围。
