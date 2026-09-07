# 前端基础库

- 根目录只保留跨领域复用且无业务状态的纯基础能力。
- `agent-display-name.ts` 是 Agent/Subagent 非空展示名称的纯所有者，只接收名称、当前翻译能力和角色，不接收 ID，不改写资源身份；真实目录和“自己”等领域判断仍由消费者负责。
- `agent-selection-options.ts` 独占 Agent 选择器的同名/缺名显示序号和缺项绑定显示；它只投影调用方已确定的候选，完整目录仅稳定序号，不能加入新候选或授予资格。目录非空名称优先，空白辅助目录不能覆盖当前领域的已知姓名。返回 value 保持精确 ID，序号不持久化，缺项当前值以禁用显示项保留。
- `unknown-value.ts` 只提供未知值的结构读取、枚举收窄和批量必填字段校验原语；领域字段集合由消费者定义。
- `agent-runtime-status.ts` 统一跨页面 Agent 运行状态解码。
- `agent-options.ts` 统一跨 Config、Settings、Contacts、Room 与 Agent 编辑器复用的 Options 默认值、目录和纯投影。
- `skill-description.ts` 只为 Nexus 随产品提供的 Skill 投影双语展示说明；不得修改传输对象或覆盖用户来源的同名 Skill。
- `skill-category.ts` 只翻译 Nexus 已知分类键；用户自定义分类始终保留服务端原名。
- `settings/` 统一 Config 与 Settings 共同依赖的偏好值清洗和 Options 合并规则。
- `avatar.ts` 统一头像标识、图标编号范围、完整字符的姓名缩写和稳定 Room 默认头像；Launcher 装饰身份也直接复用，不另算首字母。
- `text-graphemes.ts` 统一 Unicode 展示字符切分；姓名缩写、Hero 测量失败回退与流式正文共用，无 `Intl.Segmenter` 时退回完整 code point，不持有排版或推进时钟。
- `seeded-avatar.ts` 把稳定资源标识投影为跨页面一致的头像颜色与静态数学曲线路径；曲线族只在数学原点生成并等比映射到 SVG 的 `50,50`，优先采用旋转对称的径向花瓣、同余谐波、玫瑰线、Lissajous、双纽线、内旋轮线、Superformula 与极坐标编织，不持有业务状态或动画生命周期。
- `format/` 按展示值类型保存无状态格式化规则，不建立聚合出口。
- API、会话与 WebSocket 等有明确协议所有权的能力归各自子目录。
- Feature 不得通过本目录建立领域转发层；消费者直接导入具体基础函数。
- Config 与 Feature 只能共同依赖基础规则，不得让基础配置反向读取 Feature 实现。
- 禁止恢复 `utils.ts` catch-all；样式类名组合直接依赖 `shared/ui/class-name.ts`。
