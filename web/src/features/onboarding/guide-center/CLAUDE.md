# 引导中心

- `guide-center-dialog.tsx` 只组合目录内容与共享 Dialog 原语，不持有独立模态生命周期。
- `guide-center-model.ts` 用描述表投影目录条目，并纯计算 Room 导航目标。
- `use-auto-start-sidebar-tour.ts` 独占首次自动启动和重置代次。
- `use-guide-center-navigation.ts` 独占跨 Launcher、DM、Room 与 Skills 的导航命令。
- `use-guide-center-controller.ts` 只注册 Sidebar Tour、组合状态和生成弹层 Props。

Tour 注册表是可变运行态，打开引导中心时必须重新读取，不得按稳定函数引用缓存注册结果。

引导中心采用 plain 目录弹窗：标题只命名目录，不再附加“选择一个区域”等操作自述；每个条目直接给出名称、短说明、完成状态和查看动作。
