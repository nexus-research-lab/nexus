# 宽侧栏视图

- 折叠栏和展开面板只表达布局差异，不读取路由、Store 或业务 API。
- 主 Tab、Nexus 入口和系统操作必须复用同一组件树。
- `sidebar-primary-tabs-model.ts` 通过 rail/panel 规则表统一主 Tab 的样式、可见标签和无障碍投影；TSX 只保留一套按钮结构。
- 消费接口由视图定义并保持窄小；新增状态先进入上层控制器投影。
