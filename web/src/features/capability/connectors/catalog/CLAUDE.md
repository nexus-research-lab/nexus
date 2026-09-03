# Connector Catalog

- 本目录负责搜索、分类、分组和列表项展示。
- 目录身份与动作归共享能力正文标题区，本目录不再维护独立 Surface Header；分组标题只表达分类，不显示可由列表直接得出的计数。
- 目录条目复用共享响应式网格，桌面显示三列，窄窗逐级收拢。
- 分类由分组标题或筛选器表达，卡片只保留名称、一个短摘要、状态和动作，不重复分类元数据。
- 两行卡片直接使用 `UiListRow` 的 `title / description / meta` 插槽，禁止为了插入状态 Badge 而复制标题与摘要排版。
- 列表组件由消费者定义窄 props，不接收完整控制器。
- 分类与分组保持纯函数，不读取 React 状态或发请求。
- 运行时目录只展示服务端 `available` Connector，并按当前真实存在的能力分类分区；禁止用 `coming_soon` 占位卡或空分类预告未实现产品。
- 分类名称、顺序和可见集合由 `connectors-categories.ts` 与目录模型持有，页面 JSX 不维护第二份名单。
- `connector-card-model.ts` 将共享连接状态投影为列表徽标和尾部动作，卡片视图不解释原始状态字段。
- Connector 在途尾部状态使用共享 `md` Spinner，静态与动作图标保持 16px；卡片不得自行拥有颜色、旋转或 reduced-motion class。
