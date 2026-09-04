# Pairings

- `pairing-model.ts` 负责完整配对集合上的筛选、统计、分组、展示文本和创建载荷。
- `use-pairings-controller.ts` 一次加载完整配对集合，状态、渠道、智能体和搜索筛选均由本地派生；写命令成功后重新同步完整集合。
- `pairing-filter-bar.tsx` 负责状态视图和必要筛选，状态数量必须基于忽略当前状态的同一筛选范围计算。
- `pairing-list.tsx` 优先展示未授权请求，其余配对按智能体分组；绑定键、Session 等内部字段只放在共享 `UiDisclosure` 承载的技术详情中。
- 配对行默认显示可读名称、状态、频道/会话类型、外部对象标识和时间；外部对象是配对的管理身份，绑定键与 Session 才留在技术详情。待处理和 Agent 分组保留数量，用于快速判断授权规模。
- 配对分组标题复用 `CapabilitySectionHeader`，每个对象表面复用 `UiPanel`，标题、摘要、技术标签与标识分别选择共享 Typography 的 control、metadata、caption 与 code 角色；不得在列表中恢复私有字号、字重、等宽组合或任意圆角。
- 更新载荷直接使用 `UpdatePairingPayload`，禁止引入 `agentId` 等协议别名。
- 同一时间只执行一个配对写命令，重复点击由 ref 同步拦截。
- 创建、更新、删除结果未知时保留 exact 配对意图，只用 owner 全量配对读模型核对目标状态；列表仍是旧状态不能证明写入未发生，也不能自动重放。当前页面在核对或用户明确开始新操作前保持写锁。
- 手动创建弹窗直接显示失败闭环；后台反馈不得被弹窗遮住。Session 标识只进入用户主动复制，不回显在成功或失败文案中。
- 手动创建弹窗使用 plain chrome；首屏只展示渠道、会话类型、外部对象、显示名称与处理 Agent，通道账号、Thread 和初始状态进入共享 `UiDisclosure` 的 section 高级区，不在标题和尾部提示重复解释匹配协议。
- 创建等待态使用共享 `md` Spinner；弹窗不得自行拼接尺寸、旋转或 reduced-motion class。
