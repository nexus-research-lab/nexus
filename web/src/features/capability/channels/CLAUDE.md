# Channels

- 根目录只保留频道与配对页面入口、共享图标和跨域频道模型。
- `catalog/` 负责频道排序、筛选、目录请求和卡片展示。
- `connection/` 负责频道配置、账号删除与扫码登录状态机。
- `pairings/` 负责 IM 配对筛选、分组、命令和编辑视图。
- 频道目录条目使用能力域共享的可见边框，保留平台用途、状态和使用事实，不展示 API/长连接实现；配对条目复用 `ChannelIcon` 作为频道身份，不再出现无图标内容行。
- 频道目录的页头、加载/失败/空态和文本层级分别服从 `CapabilityPageLayout`、`UiResourceState` / `UiStateBlock` 与共享 Typography；不得再增加目录私有 spinner、字号或标题栏。
- 配对目录的状态切换必须复用 `CapabilityDirectoryTabs` 的中性底线，不得恢复撑满胶囊筛选；首次空态和筛选无结果都必须投影到 `UiResourceState`，业务层只决定图标、文案和动作资格，不得手写另一套居中标题、说明和按钮几何。
- API 字段直接使用后端协议命名，不在视图层维护兼容别名。
- 所有写命令通过 ref 互斥入口执行；列表请求使用请求号拒绝过期响应。
- Channel/Pairing 写失败只消费 FailureCore 的机器证据，不展示服务端、Provider、runtime 原文或内部 ID；`unknown/accepted/committed` 只允许读取核对，未证明前锁住当前页面的重复写，用户明确“开始一次新操作”才解除。
- 目录读取失败保留上次可靠的 Channel、Pairing 与 Agent 快照；首次读取失败必须显示完整的结果、数据影响和重新加载动作，不能伪装成空目录。
