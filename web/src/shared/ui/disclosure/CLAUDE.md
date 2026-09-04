# Disclosure foundation

`UiDisclosure` 是 App chrome 中原生 `details/summary` 的唯一普通实现，统一展开箭头、焦点、hover、排版和正文边界。

- `variant="panel"`：完整有框信息块，例如连接说明和高级设置。
- `variant="row"`：列表中的可展开记录，例如一次运行历史。
- `variant="section"`：表单或卡片内部由分隔线开始的次级设置。
- `variant="inline"`：正文中的低层级补充信息。
- `density` 只选择共享高度与 padding；业务层不得覆盖 summary 的圆角、颜色、字号、hover、focus 或箭头。
- `inset="sm"` 只用于卡片内部 section 与卡片左右边界对齐，不允许业务层重新拼 summary padding。
- `className` 与 `contentClassName` 只补外部 margin、grid、space、滚动和换行布局，不得重写基础视觉状态。
- Canvas 命中区、浏览器原生文件预览等非普通 App disclosure 必须在所属 L4 文档明确例外，不能复制本组件样式。
