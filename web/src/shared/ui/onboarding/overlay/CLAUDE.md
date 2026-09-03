# overlay/ - 引导浮层

- `tour-overlay.tsx` 只编排 Portal、遮罩、目标高亮和步骤卡片。
- `use-tour-overlay-layout.ts` 负责目标、卡片与视口的测量生命周期；目标滚动、窗口变化和目标自身尺寸变化必须更新同一份矩形与视口快照，无锚点的居中步骤也必须随窗口缩放重新定位。
- `tour-overlay-geometry.ts` 用定位策略表计算卡片位置，不读取 React 状态或查询 DOM。
- `tour-overlay-card.tsx` 只渲染步骤内容与导航，条目图标由完整描述表分派；标题、说明、条目与进度分别消费 `pageTitle`、`supporting`、`metadata` 与 `caption`，不得重新拼字号。目标高亮的圆角、描边、遮罩阴影和动效统一属于 `theme-recipes.css` 的 `tour-target-highlight`，Overlay 只负责测量后的几何位置。
