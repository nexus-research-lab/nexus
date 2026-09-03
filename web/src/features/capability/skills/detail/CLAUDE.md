# 技能详情

- `skill-detail-route.tsx` 只把路由参数、市场命令和详情控制器接到纯视图。
- `use-skill-detail-controller.ts` 独占详情与 Agent 使用矩阵请求代次、更新后重载、删除后导航和局部动作状态；Agent 开关通过独立原子接口切换，并通知全局目录刷新使用数。
- `skill-detail-model.ts` 用标签化快照表达加载、失败和就绪状态，集中来源、数学曲线身份、徽标、链接投影和 Markdown 前导内容转换；内置 Skill 只接收共享展示层给出的本地化说明，不改写详情资源。
- 来源、分类、徽标与 Agent 使用状态由纯模型按当前语言投影；README、标签和未知用户分类保持原始内容。
- `skill-detail-view.tsx` 只渲染模型、Agent 使用矩阵和触发命令，不直接调用 API、维护 Effect 或复制市场反馈；长说明进入 `CapabilityDetailSplitLayout` 正文列，徽标与使用矩阵进入配置侧栏，窄窗由公共布局把配置放到正文前；使用矩阵保留系统托管、主智能体、独立开关和不可配置等行为差异，名称、状态与说明共同支持决策。
- 窄屏返回语义由应用页面 Header 提供；桌面内容轴与“技能 / 当前对象”导航统一由 `CapabilityDetailPage` 持有，并与原生窗口控件中线对齐；正文保持受控阅读宽度。
- 详情返回/来源动作复用共享 Button/LinkButton，身份、标题、说明和矩阵元数据使用 App Typography；Agent 加载、空态及失败统一由 Resource State 表达，不允许私有错误卡片。
- `skill-markdown.tsx` 只把纯模型归一化后的正文交给共享 Markdown 视图。
- 更新与删除必须复用市场操作控制器；命令返回明确成功结果，失败时不得继续刷新详情或离开路由。
