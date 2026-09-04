# Launcher Hero

- `launcher-hero-stage.tsx` 只渲染首屏、输入框和最近入口。
- `launcher-recent-entry-model.ts` 只投影 DM/Room 标签、可访问名称与截断提示，不得返回 class、style、颜色、尺寸、阴影或动画参数。
- `launcher-recent-entry-layout.ts` 只拥有 Hero 最近入口的排列和渐入时序；按钮形状、尺寸、字阶与交互状态仍归共享 Button。
- `launcher-recent-entries.tsx` 只编排最近入口与主 Agent 交接动作；两类动作固定复用 `UiButton`，标签式动作通过语义 `shape="pill"` 取胶囊外形，入口说明复用 `shared/ui/overlay/tooltip`，不得恢复原生按钮、固定浅色或局部层级的 Tooltip。
- `use-launcher-query-input.ts` 拥有受控输入、IME、Mention 和提交交互。
- `use-launcher-stage-scale.ts` 拥有唯一的响应式缩放系数；云朵画布上移居中（锚点 40%）、Token 堆锚定视口底部共用系数，禁止再引入断点补丁 CSS。
- `pile/` 独立拥有 Agent Pile 的描述表、Matter 生命周期和 Token 视图，不回流 Console。
- Surface Theme 只投影 CSS 变量。

Hero 不直接调用 Launcher、Room 或 Agent API。输入匹配和插入复用 `shared/ui/mention/`，本目录只决定触发符对应的目标分类。

Launcher 查询提交使用共享 `md` Spinner 并继承提交按钮颜色；Hero 不再用边框 div 自制加载动画。
