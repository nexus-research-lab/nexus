# WorkGraph 方法论模板与产物契约

WorkGraph 模板是一套可复用的责任拓扑，不是把方法论名称贴在几个节点上。每个模板都要同时定义：问题如何被拆开、哪些判断必须有证据、什么条件才算通过，以及最后交付哪些可以复查的文件。

## 产物的四层结构

每个模板的终端节点都应产出一个交付包。交付包遵循同一套分层：

| 层 | 默认格式 | 作用 | 是否可以作为唯一事实来源 |
| --- | --- | --- | --- |
| 解释与结论 | Markdown | 给人阅读的背景、推理、建议、限制与下一步 | 否，必须引用结构化来源 |
| 结构化真相 | JSON/YAML | 表达对象、字段、假设、决策、状态和稳定 ID | 是，机器读取与复算的权威来源 |
| 审计投影 | CSV | 方便筛选、对账、逐条检查关系与证据 | 否，是结构化真相的可审计投影 |
| 可视化投影 | Mermaid/HTML | 帮助理解流程、关系和反馈回路 | 否，不能脱离结构化真相单独保存 |

模板节点的 `deliverable` 要写清文件名、格式、必需章节、证据要求和验收条件。Markdown 负责解释，JSON/YAML 负责模型，CSV 负责审计，Mermaid 负责展示；四者不能互相冒充。

这些模板不共用一条固定流水线。第一性原理的关键是从事实和反例回到约束，MECE 的关键是问题树审计与证据线汇合，双钻的关键是发散/收敛切换，OODA 的关键是快速反馈，本体论的关键是来源、治理和行动回写。WorkGraph 只提供依赖、交付和验收的宽松承载；方法本身决定分支、复核和回路，失败时可以在同一 Execution 追加下一轮节点。

## 内置模板目录

### 策略与决策

**第一性原理 `/first-principles`**

- 结构：问题界定 → 假设清单 → **事实核验与反例挑战两条线** → 从零重建方案 → 实验验证 → 独立复核 Gate → 决策记录；失败时回到下一轮重建。
- 主要产物：`first-principles.md`、`assumptions-and-constraints.csv`、`experiment-log.md`、`decision-record.md`。
- 验收：每个关键假设都有事实、来源或实验；方案能从约束推导出来；未知项和停止条件显式保留。

**麦肯锡式结构化战略 `/mece-strategy`**

- 结构：问题界定 → MECE 问题树 → **问题树完整性审计** → 假设与分析工作包 → **证据分析线与反方分析线并行** → 综合 → 逻辑审计 → 建议与执行条件。
- 主要产物：`strategy-brief.md`、`issue-tree.json`、`hypothesis-evidence-matrix.csv`、`risks-and-conditions.md`。
- 验收：问题树分支互斥且穷尽到足以支持决策；每个核心假设都有证据状态；结论、风险和行动条件一一对应。

**系统思维 `/systems-thinking`**

- 结构：系统边界 → 对象/存量/流量 → **实际行为对照与因果解释挑战并行** → 反馈回路/杠杆点 Gate → 干预方案 → 观察与学习。
- 主要产物：`systems-model.json`、`feedback-loops.csv`、`leverage-points.md`、`intervention-plan.md`、可选 `systems-map.mermaid`。
- 验收：边界、时间尺度和关键变量清楚；正负反馈与延迟被标注；每项干预都有预期指标、副作用和复盘周期。

**金字塔表达 `/pyramid-brief`**

- 结构：先给结论 → 归类论点 → **证据支持与反方论点并行** → 逻辑审计 → 高管简报。
- 主要产物：`executive-brief.md`、`argument-evidence-matrix.csv`、`logic-audit.md`。
- 验收：结论可以被一组互不重复的论点支撑；每条论点都有证据或明确标记为判断；逻辑跳跃与反例被记录。

### 研究与产品

**深度研究 `/deep-research`**

- 结构：研究问题 → 子问题和搜索策略 → 权威证据/对照证据 → 充分性评估 → 综合 → 独立核验 → 带引用报告。
- 主要产物：研究简报、累计证据矩阵、核验记录和最终带引用报告。
- 验收：重要结论可回溯到来源；冲突、缺口与不确定性仍可见；不足时必须改变策略后再进入下一轮。

**双钻设计 `/double-diamond`**

- 结构：Discover 发散探索与替代视角 → Define 收敛定义 → Develop 发散方案 → **原型测试与包容性/可行性复核并行** → Deliver 收敛交付，并允许回到前一阶段重测。
- 主要产物：`design-brief.md`、`research-insights.csv`、`prototype-test-log.md`、`delivery-decision.md`。
- 验收：洞察来自证据；选择标准先于方案选择；原型测试记录任务、观察、问题和下一步决策。

**JTBD 需求发现 `/jtbd-discovery`**

- 结构：情境与进展 → 访谈证据 → **Job 陈述与证伪线并行** → 推拉力量汇合 → 机会验证 → 验证决策复核 → 交付。
- 主要产物：`jtbd-brief.md`、`interview-evidence.csv`、`forces-of-progress.json`、`validation-record.md`。
- 验收：Job 描述用户想完成的进展而非功能；证据与解释分开；机会和验证指标能回到具体情境。

### 商业与交付

**商业模式 `/business-model`**

- 结构：九块画布 → **一致性检查与经济结构压力测试并行** → 假设实验 → **正向证据与反向测试并行** → 继续/调整/停止。
- 主要产物：`business-model.json`、`business-model.md`、`assumption-test-ledger.csv`、`continue-adjust-stop.md`。
- 验收：九块之间的关键依赖显式；最危险的假设先测试；每次实验有阈值和后续动作。

**构建与交付 `/build-ship`**

- 结构：范围 → 设计 → 实现 → 验证/复审 → 交接。
- 主要产物：变更简报、实现成果、验证证据和交接记录。
- 验收：交付物与通过验证的版本一致，阻塞项关闭或明确延期。

**复盘改进 `/review-improve`**

- 结构：基线 → 质量审计/体验审计 → 优先级 → 修订 → 复验 → 改进交付。
- 主要产物：基线、问题清单、前后对比证据、复验结论和交接记录。
- 验收：改进可测量且无重要回归；评价标准失效时回到基线重建。

### 运营与本体

**OODA 循环 `/ooda-loop`**

- 结构：Observe 观察 → Orient 定向 → **行动选项与红队挑战并行** → Decide 决策 → Act 行动 → 反馈回到下一轮观察。
- 主要产物：`observation-orientation-log.csv`、`ooda-decision.md`、`action-feedback-log.csv`。
- 验收：决策使用的观察窗口和假设清楚；行动有负责人、截止时间和反馈指标；每轮可继续、暂停或结束。

**本体论运营模型 `/ontology-model`**

- 结构：对象类型 → 属性/关系 → **来源一致性审计** → 动作/函数 → **权限治理复核** → 真实业务用例与结果回写。
- 主要产物：`ontology.yaml`（canonical）、`ontology-model.md`、`object-link-matrix.csv`、`evidence-lineage.csv`、`action-governance.md`，以及可选 `ontology-map.mermaid`。
- 验收：对象、属性、关系、动作和权限都有稳定 ID；每个重要字段可追溯到来源；动作说明输入、前置条件、副作用和审计；至少一个真实运营用例能从对象读到行动再回写结果。
- 说明：该模板借鉴公开的 operational ontology / digital twin 思路，不声称复刻 Palantir Foundry 的私有实现。YAML/JSON 是权威模型，关系图只是投影。

## 模板如何进入运行

目录卡片只显示 Slash、名称、来源和节点数。打开详情后显示目标、责任拓扑、完成条件和产物契约；真正执行时，终端节点必须按契约写出交付包并提交证据，不能只返回一段总结。内置模板只读，owner 保存的模板可以继续编辑，但每次复用都会创建新的 Execution、Work Item、交付和验收身份。

## 公开参考

- [Design Council · Framework for Innovation](https://www.designcouncil.org.uk/resources/framework-for-innovation/)：双钻的发散与收敛。
- [Christensen Institute · Jobs to Be Done](https://www.christenseninstitute.org/theory/jobs-to-be-done/)：情境中的用户进展与推拉力量。
- [Strategyzer · Business Model Canvas](https://www.strategyzer.com/library/the-business-model-canvas)：九块商业模式画布。
- [Barbara Minto](https://www.barbaraminto.com/)：金字塔式结论、论点和证据组织。
- [Donella Meadows · Leverage Points](https://donellameadows.org/archives/leverage-points-places-to-intervene-in-a-system/)：系统杠杆点与干预。
- [Palantir Foundry · Ontology overview](https://www.palantir.com/docs/foundry/ontology/overview/) 与 [Core concepts](https://www.palantir.com/docs/foundry/ontology/core-concepts/)：对象、属性、关系、动作和治理的公开概念；本模板只借鉴公开思想。
