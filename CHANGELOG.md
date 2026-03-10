# Changelog

## 2026-03-10

### Changed
- Web 首页重构为 B 端控制台骨架，采用 `Agent Directory -> Agent Space` 的两层信息架构。
- 在 Agent Space 内新增顶部快速切换器，支持不返回目录页直接切换 Agent，兼顾结构分层与多线程操作效率。
- 新增 Session Rail 与 Agent Inspector，为后续承接权限队列、运行审计、Workspace/Memory 面板预留结构位置。
- 全局设计 token 从赛博终端风格收口为更稳定的控制台视觉语言，统一页面底色、字体和 panel 表达。
- 首页文案进一步收口为简洁后台表达，移除解释型和营销式描述。
- 首页主色调整为更克制的深灰蓝方案，降低装饰性和视觉噪声。
- Agent Space 布局进一步压缩，减少头部与侧栏占位，把更多空间留给会话主区和实际可操作面板。
- 新增 workspace 文件浏览、读取和写回能力，右侧面板可直接操作本地 workspace 文件，并在保存后刷新 Agent 活跃 session。

## 2026-03-09

### Changed
- 收口 Session 边界：Session API 不再承载执行配置，执行参数统一归 Agent 管理。
- 默认 Agent 路由改为配置化，通过 `DEFAULT_AGENT_ID` 显式控制默认路由策略。
- Agent 更新配置或 workspace 后，会主动失效内存中的活跃 SDK session，后续消息按最新 Agent 配置懒加载重建。

### Docs
- 在仓库规则中补充“一个完整需求对应一个提交”和“必要时同步更新 changelog”的协作约定。
