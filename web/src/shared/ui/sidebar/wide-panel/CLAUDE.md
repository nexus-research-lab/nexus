# Sidebar Wide Panel

## 分层

- `sidebar-wide-panel.tsx`：读取路由、认证和 Sidebar Store，装配共享数据。
- `sidebar-collapsed-rail.tsx`、`sidebar-expanded-panel.tsx`：只处理布局差异。
- `sidebar-primary-tabs.tsx`、`sidebar-nexus-button.tsx`、`sidebar-utility-actions.tsx`：折叠与展开共用的交互组件。

## 约束

- 折叠和展开状态不得复制 Tab 或系统操作按钮树，视觉差异通过有限 `variant` 表达。
- 路由到主 Tab 的映射保持单一来源；新增 Tab 时同时补齐映射、动作和内容组件表。
- 引导中心只挂载一次；布局分支不得各自持有弹层实例。
