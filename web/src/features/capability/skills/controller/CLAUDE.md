# 技能市场控制器

## 职责

- `use-skill-marketplace.ts` 只装配各状态域和发现模式。
- `use-skill-catalog.ts` 独占目录查询、分类归一化和目录派生数据。
- `use-external-skill-search.ts` 独占外部搜索、服务端来源作用域、预览和请求竞态。
- `use-external-skill-sources.ts` 独占来源清单、私有来源增删改、开关动作和搜索修订号；写响应成功与后续列表刷新是两个阶段，刷新失败不能把已保存更改误报为失败，结果未知只能先刷新对账。
- `use-skill-operations.ts` 独占导入、更新、删除和定时更新检查；写响应与目录刷新分阶段，`skill-operation-recovery.ts` 只按 exact 操作、Skill/来源和权威详情核对结果。
- `skill-deploy-failures.ts` 属于操作反馈，不能从详情页反向引入；它保留更新已成功/部署部分失败与准确数量，只显示前三个真实姓名或共享通称，不暴露 Agent ID 或原始 runtime 错误。
- `skill-update-check-model.ts` 把批量检查结果投影为明确的 current / updates / failure 通知；视图不得通过匹配文案推断状态，部分失败不得伪装成“暂无更新”。
- `use-skill-marketplace-feedback.ts` 用单一反馈状态表达处理中、成功、部分完成和失败。

## 不变量

- 并发技能动作使用 `Set` 表达 busy 集合，不得退回单个名称哨兵。
- 更新与删除命令返回明确成功结果，路由后续动作不得根据 Promise 已结束推断命令成功。
- 目录、搜索和预览只允许最新请求提交结果。
- 操作失败必须替换进行中反馈，禁止同时残留互相矛盾的消息。
- controller 返回具体状态模型；消费者自行定义窄 Props，不依赖完整控制器类型。
- 外部技能身份、来源和导入展示状态属于 `external/` 纯模型；controller 只管理请求与命令生命周期。
- 外部搜索、预览和来源开关的缺省反馈必须使用当前界面语言；第三方或服务端返回的具体错误仍作为诊断原因保留。
- Skill 写结果为 `unknown` 时，当前页面只锁定同一 exact 意图；重新点击和反馈动作只能读取目录/详情，不能重放写入。权威读取仍不能证明结果时，必须由用户显式开始新意图后才解锁。
- 写响应已证明 `committed` 后，目录刷新失败只表示当前列表可能过期；恢复动作只能刷新目录，不能把已完成写入误报为失败或再次提交。

- 目录 controller 保存独立读取失败事实，只有最新成功读取清除；失败与在途读取不丢弃上一份技能快照，目录视图不得把失败解释为服务端空结果。

- 来源读取恢复与 mutation 互斥：内部提交后的刷新显式获准，其他刷新不与在途写交叉。成功列表只证明读取完成，不能证明凭据轮换等未知写结果；recovery effect 独立保存，不依赖 feedback 是否可见。社区搜索独立保存 loadFailed，并以同一 submitted query/source 重读恢复。

- 来源 mutationBlocked 与实际 loading 分开输出：未知结果禁用写控件，但不能假装仍在读取或阻止社区来源的只读使用。
