# Pairings

- `pairing-model.ts` 负责完整配对集合上的筛选、统计、分组、展示文本和创建载荷。
- `use-pairings-controller.ts` 一次加载完整配对集合，状态、渠道、智能体和搜索筛选均由本地派生；写命令成功后重新同步完整集合。
- `pairing-filter-bar.tsx` 负责状态视图和必要筛选，状态数量必须基于忽略当前状态的同一筛选范围计算。
- `pairing-list.tsx` 优先展示未授权请求，其余配对按智能体分组；绑定键、Session 等内部字段只放在可展开的技术详情中。
- 更新载荷直接使用 `UpdatePairingPayload`，禁止引入 `agentId` 等协议别名。
- 同一时间只执行一个配对写命令，重复点击由 ref 同步拦截。
