# 技能详情

- `skill-detail-route.tsx` 只把路由参数、市场命令和详情控制器接到纯视图。
- `use-skill-detail-controller.ts` 独占详情请求代次、更新后重载、删除后导航和局部动作状态。
- `skill-detail-model.ts` 用标签化快照表达加载、失败和就绪状态，并集中来源、图标、徽标和链接投影。
- `skill-detail-view.tsx` 只渲染模型和触发命令，不直接调用 API、维护 Effect 或复制市场反馈。
- 更新与删除必须复用市场操作控制器；命令返回明确成功结果，失败时不得继续刷新详情或离开路由。
