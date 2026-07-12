# Channel Catalog

- `channel-catalog-model.ts` 负责排序、筛选，以及卡片动作、徽标、元数据和统计项的完整投影。
- `use-channels-controller.ts` 持有频道、Agent、选中项和目录反馈。
- 选中项由 `channel_type` 从最新列表派生，不复制完整频道对象。
- 卡片视图只消费判别式动作和可直接渲染的条目，不得重新解释协议状态。
