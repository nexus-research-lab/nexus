# 能力侧栏

- `capability-sidebar-model.ts` 用定义表生成导航项并负责搜索投影，不读取 React 状态。
- `capability-sidebar-item.tsx` 只呈现能力导航项，不承载通用重命名、删除或业务外观扩展。
- `use-capability-summary.ts` 统一摘要加载、窗口重验证和变更事件订阅；请求合并规则归 `capability-summary-refresh-model.ts`。
- `capability-sidebar-panel.tsx` 只组合查询、摘要、路由导航和空状态。
- 本子域不提供旧路径转发；新增能力入口必须先进入导航定义表。
