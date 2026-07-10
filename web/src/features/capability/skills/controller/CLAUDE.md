# 技能市场控制器

## 职责

- `use-skill-marketplace.ts` 只装配各状态域和发现模式。
- `use-skill-catalog.ts` 独占目录查询、分类归一化和目录派生数据。
- `use-external-skill-search.ts` 独占外部搜索、预览和请求竞态。
- `use-external-skill-sources.ts` 独占来源清单、开关动作和搜索修订号。
- `use-skill-operations.ts` 独占导入、更新、删除和定时更新检查。
- `use-skill-marketplace-feedback.ts` 用单一反馈状态表达处理中、成功、部分完成和失败。

## 不变量

- 并发技能动作使用 `Set` 表达 busy 集合，不得退回单个名称哨兵。
- 目录、搜索和预览只允许最新请求提交结果。
- 操作失败必须替换进行中反馈，禁止同时残留互相矛盾的消息。
- controller 返回具体状态模型；消费者自行定义窄 Props，不依赖完整控制器类型。
