# Channel Catalog

- `channel-catalog-model.ts` 只负责排序、筛选与卡片文案派生。
- `use-channels-controller.ts` 持有频道、Agent、选中项和目录反馈。
- 选中项由 `channel_type` 从最新列表派生，不复制完整频道对象。
