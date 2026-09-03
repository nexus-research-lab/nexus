# 按钮原语

- 本目录拥有按钮和图标按钮的交互结构及样式投影。
- 业务命令、权限判断和加载事务由消费者负责。
- `button-styles.ts` 是 primitive 内部视觉投影；`features/pages` 必须渲染 `UiButton / UiLinkButton / UiIconButton`，不得导入样式函数再手写原生 DOM。需要的 size/tone/variant 类型由 `button.tsx` 一并导出。
- `UiIconButton` 用显式 `tooltip`、`title` 或字符串 `aria-label` 驱动共享 Tooltip；原生 `title` 不再下发给按钮，避免两套悬浮提示叠加。
- `UiIconButton` 默认使用随尺寸变化的控件圆角；只有导航返回、更多操作等明确的圆形图标动作才传 `shape="round"`。业务层不得用 `rounded-*` 覆盖形状。
- Button 尺寸直接映射 App Typography：`2xs / xs / sm` 分别承载 24px 微型工具条、28px 紧凑动作和 32px 次级动作，并使用 caption/metadata；普通 `md / lg` 使用 14px control。IconButton 的 `2xs / xs / sm / md / lg` 固定为 20/24/28/32/36px；业务层不得用 `text-* / leading-* / font-*` 覆盖。
- Ghost 与 text 按钮默认透明并使用次级文字，hover 使用轻中性底，checked / current-page / expanded / pressed state 使用更明确的中性活动底；状态切换不得增加边框或改变几何。品牌色只归明确的 primary 动作，危险色只归 destructive 动作，`success` 只表达已完成的短暂确认状态（例如复制成功），不能代替常规选中态。
- `button.test.tsx` 使用真实表单、链接、图标动作、点击与键盘焦点证明默认 `type=button`、显式 submit、disabled、导航语义和可访问名称合同；业务层不得以源码断言替代这些行为测试。
