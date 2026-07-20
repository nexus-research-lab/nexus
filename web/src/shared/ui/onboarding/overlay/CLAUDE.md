# overlay/ - 引导浮层

- `tour-overlay.tsx` 只编排 Portal、遮罩、目标高亮和步骤卡片。
- `use-tour-overlay-layout.ts` 负责目标与卡片的测量生命周期；目标滚动、窗口变化和目标自身尺寸变化必须更新同一份矩形快照。
- `tour-overlay-geometry.ts` 用定位策略表计算卡片位置，不读取 React 状态或查询 DOM。
- `tour-overlay-card.tsx` 只渲染步骤内容与导航，条目图标由完整描述表分派。
