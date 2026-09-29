// INPUT: 抽象方法论的责任阶段、依赖关系与可复核的产物契约。
// OUTPUT: 只读双语内置 WorkGraph 模板，供目录、Slash catalog 和 runtime 展开。
// POS: 方法论模板的唯一代码语义；不携带 owner、Execution 或运行结果。
package workgraphworkflow

import "github.com/nexus-research-lab/nexus/internal/protocol"

// methodologyBuiltinWorkflowDefinitions keeps the reusable method templates
// separate from the original execution-oriented templates. Each definition
// carries an artifact contract so a terminal node has a concrete delivery
// package, rather than only a graph label.
var methodologyBuiltinWorkflowDefinitions = []builtinWorkflowDefinition{
	{
		slashName: "first-principles", title: lt("First Principles", "第一性原理"),
		description:        lt("Use this when a familiar solution may hide bad assumptions: separate facts, constraints, and unknowns, then rebuild and test the decision. It records the reasoning chain, assumption ledger, experiments, and final trade-offs.", "当一个看似熟悉的方案可能建立在错误前提上时使用：把事实、约束和未知项拆开，从底层重建并测试决策。最终会留下推理链、假设台账、验证实验和带权衡的决策记录。"),
		objective:          lt("Reach a decision that can be reconstructed from explicit assumptions, facts, constraints, and experiments.", "形成一项可以从明确假设、事实、约束和实验重新推导的决策。"),
		completionCriteria: []localizedWorkflowText{lt("Every material assumption is supported, tested, or explicitly unresolved.", "每个重要假设都有支持、测试或明确的未决状态。"), lt("The chosen option follows from the constraints and survives a validation check.", "选定方案能够由约束推导出来，并通过验证检查。")},
		artifactContract:   ac("first-principles", "ontology of assumptions and constraints is canonical; Markdown explains the reasoning", "show a derivation spine with evidence and experiment gates", artifact("first-principles.md", "reasoning record", "Markdown", "Human-readable derivation and decision record", "problem framing", "assumptions", "constraints", "rebuild", "decision"), artifact("assumptions-and-constraints.csv", "assumption ledger", "CSV", "Auditable fact, assumption, source, and status rows", "id", "statement", "type", "evidence", "status"), artifact("experiment-log.md", "validation log", "Markdown", "Tests, measurements, results, and interpretation", "hypothesis", "method", "result", "interpretation"), artifact("decision-record.md", "decision record", "Markdown", "Decision, trade-offs, and unresolved risks", "decision", "alternatives", "trade-offs", "risks")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("frame", protocol.WorkItemKindProduce, lt("Frame the problem", "界定问题"), lt("State the decision, desired outcome, scope, and evidence boundary.", "说明决策、目标结果、范围和证据边界。"), lt("A problem frame with a falsifiable success condition.", "包含可证伪成功条件的问题界定。"), false),
			methodNode("assumptions", protocol.WorkItemKindProduce, lt("List assumptions and constraints", "列出假设与约束"), lt("Separate observed facts, assumptions, non-negotiable constraints, and unknowns.", "区分观察事实、假设、不可违背约束和未知项。"), lt("The assumptions-and-constraints.csv ledger.", "assumptions-and-constraints.csv 台账。"), false),
			methodNode("facts", protocol.WorkItemKindVerify, lt("Verify the bottom-layer facts", "核验底层事实"), lt("Check each material fact against a source, measurement, or reproducible calculation.", "让每个重要事实回到来源、测量或可复现计算。"), lt("A fact-check record with confidence and unresolved unknowns.", "包含置信度与未决未知项的事实核验记录。"), false),
			methodNodeRole("counterexamples", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Challenge assumptions with counterexamples", "用反例挑战假设"), lt("Search for cases that would break the assumed constraint or invalidate the proposed reasoning.", "寻找会打破约束或推翻当前推理的案例。"), lt("A counterexample and falsification register.", "反例与证伪登记。"), false),
			methodNode("rebuild", protocol.WorkItemKindProduce, lt("Rebuild candidate solutions", "从底层重建候选方案"), lt("Derive candidate options from the accepted facts and constraints without importing untested convention.", "只从已接受事实和约束推导候选方案，不把未经验证的惯例当成前提。"), lt("A derivation-backed option set with explicit trade-offs.", "有推导依据且明确权衡的候选方案集。"), false),
			methodNode("validate", protocol.WorkItemKindVerify, lt("Run validation experiments", "执行验证实验"), lt("Test the highest-risk assumptions and calculate the outcome against the success condition.", "测试风险最高的假设，并按成功条件计算结果。"), lt("The experiment-log.md validation record.", "experiment-log.md 验证记录。"), false),
			methodNodeRole("review", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Independently review the derivation", "独立复核推导"), lt("Check that the experiment actually tests the stated assumption and that the result supports the conclusion.", "检查实验是否真的测试了假设，以及结果是否足以支撑结论。"), lt("A review gate with corrections or a confirmed decision path.", "带修正项或确认结论路径的复核 Gate。"), false),
			methodNode("decide", protocol.WorkItemKindIntegrate, lt("Record the decision", "记录决策"), lt("Select, reject, or defer an option and preserve the reasoning chain and unresolved risks.", "选择、否决或暂缓方案，并保留推理链与未解决风险。"), lt("The first-principles.md and decision-record.md delivery package.", "first-principles.md 与 decision-record.md 交付包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("assumptions", "frame"), dep("facts", "assumptions"), dep("counterexamples", "assumptions"),
			dep("rebuild", "facts"), dep("rebuild", "counterexamples"), dep("validate", "rebuild"),
			dep("review", "validate"), dep("decide", "review"),
		},
	},
	{
		slashName: "mece-strategy", title: lt("MECE Strategy", "麦肯锡式结构化战略"),
		description:        lt("Use this for an ambiguous business or strategy question that must become a decision: split it into an MECE issue tree, test the most important hypotheses, and challenge the recommendation. It records the issue tree, evidence matrix, risks, and an action-ready strategy brief.", "当一个模糊的业务或战略问题需要变成明确决策时使用：先拆成 MECE 问题树，再验证关键假设并挑战建议。最终会留下问题树、证据矩阵、风险与前提，以及可以直接用于决策的战略简报。"),
		objective:          lt("Turn an ambiguous strategic question into a decision-ready recommendation with traceable hypotheses, evidence, and conditions.", "把模糊的战略问题转化为带有可追溯假设、证据和前提条件的可决策建议。"),
		completionCriteria: []localizedWorkflowText{lt("The issue tree covers the decision without material overlap or blind spots.", "问题树覆盖决策所需范围，没有重要重叠或盲区。"), lt("Each critical hypothesis has evidence, a counterargument, and an implication.", "每个关键假设都有证据、反方挑战和行动含义。")},
		artifactContract:   ac("mece-strategy", "issue-tree.json and hypothesis-evidence-matrix.csv are canonical", "show a left-to-right issue tree with evidence status badges", artifact("strategy-brief.md", "strategy brief", "Markdown", "Decision, recommendation, rationale, and implementation conditions", "decision", "issue tree", "recommendation", "risks", "next steps"), artifact("issue-tree.json", "issue tree", "JSON", "Stable MECE branches and their decision questions", "root", "branches", "questions", "coverage"), artifact("hypothesis-evidence-matrix.csv", "evidence matrix", "CSV", "Hypotheses, evidence, confidence, and implications", "hypothesis", "source", "status", "confidence", "implication"), artifact("risks-and-conditions.md", "risk register", "Markdown", "Counterarguments, risks, and conditions for action", "risk", "counterargument", "condition", "owner")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("question", protocol.WorkItemKindProduce, lt("Frame the strategic question", "界定战略问题"), lt("Define the decision, audience, time horizon, and what a useful answer must decide.", "定义决策、受众、时间范围，以及有用答案必须解决什么。"), lt("A strategy brief with a decision question and scope.", "包含决策问题与范围的战略简报。"), false),
			methodNode("tree", protocol.WorkItemKindProduce, lt("Build the MECE issue tree", "建立 MECE 问题树"), lt("Partition the question into mutually exclusive branches that collectively cover the decision.", "把问题划分为相互独立且足以覆盖决策的分支。"), lt("The issue-tree.json model with coverage notes.", "带覆盖说明的 issue-tree.json 模型。"), false),
			methodNode("tree-audit", protocol.WorkItemKindVerify, lt("Audit tree completeness", "审计问题树完整性"), lt("Test the tree for overlap, missing branches, ambiguous labels, and a clear path to the decision.", "检查问题树是否有重叠、遗漏、含糊标签，以及是否能通向决策。"), lt("A MECE coverage verdict and corrected tree.", "MECE 覆盖结论与修正后的问题树。"), false),
			methodNode("hypotheses", protocol.WorkItemKindProduce, lt("Set hypotheses and work packages", "设定假设与分析工作包"), lt("State testable hypotheses, required analyses, owners, and disconfirming evidence.", "写出可测试假设、所需分析、负责人和反证。"), lt("A prioritized hypothesis work plan.", "有优先级的假设分析计划。"), false),
			methodNode("evidence", protocol.WorkItemKindProduce, lt("Run the evidence workstreams", "执行证据分析线"), lt("Collect and analyze evidence for each hypothesis with explicit confidence and implication.", "为每个假设收集并分析证据，明确置信度与行动含义。"), lt("A hypothesis-evidence matrix with evidence status.", "带证据状态的假设证据矩阵。"), false),
			methodNodeRole("countercase", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Run the countercase workstream", "执行反方分析线"), lt("Independently seek counterevidence, alternative explanations, and conditions under which the recommendation fails.", "独立寻找反证、替代解释，以及建议失效的条件。"), lt("A countercase register with risks and conditions.", "带风险与前提的反方登记。"), false),
			methodNode("synthesis", protocol.WorkItemKindIntegrate, lt("Synthesize the case", "综合论证"), lt("Reconcile evidence and countercases into a balanced answer without hiding uncertainty.", "把证据与反方论点整合为平衡答案，不隐藏不确定性。"), lt("A draft recommendation with explicit confidence and gaps.", "包含置信度与缺口的建议草稿。"), false),
			methodNode("logic-audit", protocol.WorkItemKindVerify, lt("Audit the recommendation logic", "审计建议逻辑"), lt("Check that the conclusion follows from the tree, evidence, and conditions, then route corrections before delivery.", "检查结论是否确实由问题树、证据和前提推出，并在交付前回送修正。"), lt("A final logic audit and resolved findings.", "最终逻辑审计与已解决问题。"), false),
			methodNode("recommend", protocol.WorkItemKindIntegrate, lt("Deliver the recommendation", "交付战略建议"), lt("Synthesize the answer, choices, implications, and execution conditions for the decision maker.", "为决策者综合答案、选择、影响和执行前提。"), lt("A decision-ready strategy-brief.md package.", "可直接决策的 strategy-brief.md 交付包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("tree", "question"), dep("tree-audit", "tree"), dep("hypotheses", "tree-audit"),
			dep("evidence", "hypotheses"), dep("countercase", "hypotheses"),
			dep("synthesis", "evidence"), dep("synthesis", "countercase"),
			dep("logic-audit", "synthesis"), dep("recommend", "logic-audit"),
		},
	},
	{
		slashName: "systems-thinking", title: lt("Systems Thinking", "系统思维"),
		description:        lt("Use this when a problem keeps returning because many actors, feedback loops, or time delays interact: map the system, compare it with observed behavior, and test the causal story before intervening. It records the system model, loop register, leverage points, and intervention plan.", "当问题反复出现，且多个参与者、反馈回路或时间延迟相互影响时使用：先画出系统并与真实行为对照，再检验因果解释，最后选择干预。最终会留下系统模型、回路登记、杠杆点分析和干预计划。"),
		objective:          lt("Choose an intervention that improves the system outcome while making feedback, delays, and side effects observable.", "选择能改善系统结果且能观察反馈、延迟和副作用的干预方案。"),
		completionCriteria: []localizedWorkflowText{lt("The system model identifies the important variables, relationships, and time horizon.", "系统模型识别出重要变量、关系和时间范围。"), lt("Intervention metrics and unintended effects have a learning loop.", "干预指标和意外影响都有学习闭环。")},
		artifactContract:   ac("systems-thinking", "systems-model.json is canonical; Mermaid is a projection", "show causal loops with polarity and delay markers", artifact("systems-model.json", "system model", "JSON", "Objects, variables, relations, boundaries, and time horizon", "boundary", "variables", "relations", "time horizon"), artifact("feedback-loops.csv", "loop register", "CSV", "Reinforcing/balancing loops, delays, and indicators", "loop", "polarity", "delay", "indicator"), artifact("leverage-points.md", "leverage analysis", "Markdown", "Candidate intervention points and expected behavior", "point", "mechanism", "strength", "risk"), artifact("intervention-plan.md", "intervention plan", "Markdown", "Actions, measures, side effects, and review cadence", "action", "metric", "owner", "review")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("boundary", protocol.WorkItemKindProduce, lt("Set the system boundary", "设定系统边界"), lt("Define actors, environment, time horizon, and what is deliberately outside the model.", "定义参与者、环境、时间范围，以及明确不纳入模型的内容。"), lt("A boundary statement and variable inventory.", "边界声明与变量清单。"), false),
			methodNode("model", protocol.WorkItemKindProduce, lt("Map stocks, flows, and relationships", "映射存量、流量与关系"), lt("Represent material variables, causal links, polarity, and delays in a structured model.", "用结构化模型表示重要变量、因果关系、正负极性和延迟。"), lt("The systems-model.json canonical model.", "systems-model.json 权威模型。"), false),
			methodNode("behavior", protocol.WorkItemKindProduce, lt("Compare observed system behavior", "对照实际系统行为"), lt("Check the model against observed patterns, trends, and boundary conditions before selecting leverage points.", "在选择杠杆点前，用实际模式、趋势和边界条件对照模型。"), lt("A behavior-to-model comparison record.", "系统行为与模型对照记录。"), false),
			methodNodeRole("loop-challenge", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Challenge the causal story", "挑战因果解释"), lt("Seek alternative explanations, missing loops, delays, and unintended consequences.", "寻找替代解释、遗漏回路、延迟和意外后果。"), lt("A causal challenge register.", "因果挑战登记。"), false),
			methodNode("loops", protocol.WorkItemKindVerify, lt("Check feedback loops and leverage points", "检查反馈回路与杠杆点"), lt("Reconcile observed behavior and challenges into a defensible loop and leverage assessment.", "把实际行为和挑战意见汇合成可辩护的回路与杠杆评估。"), lt("A feedback loop register and leverage analysis.", "反馈回路登记与杠杆分析。"), false),
			methodNode("intervene", protocol.WorkItemKindIntegrate, lt("Plan the intervention and learning loop", "规划干预与学习闭环"), lt("Choose an intervention, measures, guardrails, and a review cadence that can detect unintended effects.", "选择干预、指标、护栏和能发现意外影响的复盘节奏。"), lt("The intervention-plan.md delivery package.", "intervention-plan.md 交付包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("model", "boundary"), dep("behavior", "model"), dep("loop-challenge", "model"),
			dep("loops", "behavior"), dep("loops", "loop-challenge"), dep("intervene", "loops"),
		},
	},
	{
		slashName: "double-diamond", title: lt("Double Diamond", "双钻设计"),
		description:        lt("Use this when the team does not yet know the right problem or solution: widen the research first, narrow to a clear opportunity, prototype alternatives, and test before delivery. It records user insights, the design brief, prototype findings, and the delivery decision.", "当团队还不知道真正的问题或最佳方案时使用：先发散研究，再收敛为清晰机会，制作多个原型并在交付前测试。最终会留下用户洞察、设计简报、原型测试记录和交付决策。"),
		objective:          lt("Turn evidence about people and context into a tested, deliverable solution with explicit decision points.", "把关于用户和情境的证据转化为经过测试、可以交付的方案。"),
		completionCriteria: []localizedWorkflowText{lt("Insights and decisions are traceable to research or tests.", "洞察和决策都能追溯到研究或测试。"), lt("The chosen delivery option passed a defined usability or outcome test.", "选定交付方案通过了预先定义的可用性或结果测试。")},
		artifactContract:   ac("double-diamond", "research-insights.csv and test log are canonical evidence", "show four stages with divergence/convergence bands", artifact("design-brief.md", "design brief", "Markdown", "Problem, audience, constraints, and success measures", "context", "problem", "audience", "success"), artifact("research-insights.csv", "insight ledger", "CSV", "Observed needs, evidence, and opportunity statements", "observation", "source", "insight", "confidence"), artifact("prototype-test-log.md", "test log", "Markdown", "Prototype tasks, observations, findings, and changes", "test", "task", "observation", "finding", "change"), artifact("delivery-decision.md", "delivery decision", "Markdown", "Selected solution, trade-offs, and handoff criteria", "option", "decision", "trade-offs", "handoff")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("discover", protocol.WorkItemKindProduce, lt("Discover the context", "Discover 探索情境"), lt("Explore people, behavior, constraints, and adjacent opportunities without prematurely converging.", "探索用户、行为、约束和相邻机会，不提前收敛。"), lt("Research notes and an insight ledger.", "研究笔记与洞察台账。"), false),
			methodNodeRole("perspectives", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Seek alternative perspectives", "寻找替代视角"), lt("Look for non-users, edge cases, and contradictory evidence before narrowing the problem.", "在收敛问题前寻找非典型用户、边界案例和矛盾证据。"), lt("A perspective and contradiction register.", "视角与矛盾登记。"), false),
			methodNode("define", protocol.WorkItemKindIntegrate, lt("Define the opportunity", "Define 定义机会"), lt("Cluster evidence and alternative perspectives into a focused problem, audience, and measurable success criteria.", "把证据和替代视角归纳成聚焦问题、目标人群和可测量成功标准。"), lt("The design-brief.md problem definition.", "design-brief.md 问题定义。"), false),
			methodNode("develop", protocol.WorkItemKindProduce, lt("Develop and test options", "Develop 形成并测试方案"), lt("Generate alternatives, prototype the riskiest assumptions, and record observed behavior.", "形成多个方案，为风险最高的假设制作原型并记录观察结果。"), lt("The prototype-test-log.md record with a chosen direction.", "带选定方向的 prototype-test-log.md 记录。"), false),
			methodNodeRole("prototype-test", protocol.WorkItemKindVerify, protocol.WorkGraphWorkflowNodeKey, lt("Test the riskiest prototype", "测试最高风险原型"), lt("Run the defined tasks and record behavior, findings, and changes.", "执行预先定义的任务，记录行为、发现和修改。"), lt("The prototype-test-log.md test record.", "prototype-test-log.md 测试记录。"), false),
			methodNodeRole("inclusion-check", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Review inclusion and feasibility", "复核包容性与可行性"), lt("Check edge cases, accessibility, operational constraints, and whether the option works beyond the happy path.", "检查边界案例、可访问性、运营约束，以及方案是否只在理想路径上成立。"), lt("An independent design review with blocking findings.", "带阻塞问题的独立设计复核。"), false),
			methodNode("deliver", protocol.WorkItemKindIntegrate, lt("Deliver the tested direction", "Deliver 交付经过测试的方向"), lt("Confirm the selected option meets the success measures and package the handoff decision.", "确认选定方案满足成功指标，并整理交接决策。"), lt("The delivery decision and ready-to-handoff design package.", "交付决策与可交接设计包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("perspectives", "discover"), dep("define", "discover"), dep("define", "perspectives"),
			dep("develop", "define"), dep("prototype-test", "develop"), dep("inclusion-check", "develop"),
			dep("deliver", "prototype-test"), dep("deliver", "inclusion-check"),
		},
	},
	{
		slashName: "jtbd-discovery", title: lt("Jobs to Be Done Discovery", "JTBD 需求发现"),
		description:        lt("Use this when users describe features but not the progress they are trying to make: study the triggering situation, desired progress, competing forces, and disconfirming interviews. It records the job statement, interview evidence, forces model, and next validation experiment.", "当用户只描述功能，却说不清自己要完成什么进展时使用：研究触发情境、期望进展、竞争力量和反例访谈。最终会留下 Job 陈述、访谈证据、进展力量模型和下一项验证实验。"),
		objective:          lt("Define a situation-based job and validate an opportunity without mistaking features for user progress.", "定义基于情境的 Job，并验证机会，避免把功能误当成用户进展。"),
		completionCriteria: []localizedWorkflowText{lt("The job statement describes progress in a situation rather than a feature request.", "Job 陈述描述情境中的进展，而不是功能请求。"), lt("Opportunity and validation metrics are grounded in interview evidence.", "机会和验证指标都建立在访谈证据上。")},
		artifactContract:   ac("jtbd-discovery", "interview-evidence.csv and forces-of-progress.json are canonical", "show a situation-to-job funnel and force balance", artifact("jtbd-brief.md", "JTBD brief", "Markdown", "Situation, progress, job statement, and opportunity", "situation", "trigger", "job", "opportunity"), artifact("interview-evidence.csv", "interview ledger", "CSV", "Quotes, situations, observed progress, and confidence", "participant", "situation", "evidence", "interpretation"), artifact("forces-of-progress.json", "forces model", "JSON", "Push, pull, anxiety, and habit forces", "situation", "push", "pull", "anxiety", "habit"), artifact("validation-record.md", "validation record", "Markdown", "Test design, threshold, outcome, and next decision", "hypothesis", "metric", "threshold", "outcome")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("context", protocol.WorkItemKindProduce, lt("Capture the situation", "记录使用情境"), lt("Describe the trigger, context, desired progress, and current workaround.", "描述触发点、情境、想取得的进展和当前替代方案。"), lt("A situation and interview plan.", "情境与访谈计划。"), false),
			methodNode("interviews", protocol.WorkItemKindProduce, lt("Collect interview evidence", "收集访谈证据"), lt("Record concrete stories, language, behavior, and outcomes while separating quotes from interpretation.", "记录具体故事、语言、行为和结果，并区分原话与解释。"), lt("The interview-evidence.csv ledger.", "interview-evidence.csv 台账。"), false),
			methodNode("job", protocol.WorkItemKindProduce, lt("State the job to be done", "写出待完成的 Job"), lt("Describe the progress someone is trying to make in the situation, without turning it into a feature.", "描述用户在情境中试图取得的进展，不把它写成功能。"), lt("A situation-based job statement.", "基于情境的 Job 陈述。"), false),
			methodNodeRole("disconfirm", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Disconfirm the job statement", "证伪 Job 陈述"), lt("Look for interviews and situations where the proposed job does not explain the choice or switching behavior.", "寻找无法解释选择或转换行为的访谈和情境。"), lt("A disconfirmation and boundary register.", "证伪与边界登记。"), false),
			methodNode("forces", protocol.WorkItemKindVerify, lt("Model the forces of progress", "建模进展力量"), lt("Reconcile push, pull, anxiety, and habit forces with the job and the disconfirming cases.", "把推动、拉动、焦虑和习惯力量与 Job 及证伪案例对照。"), lt("The forces-of-progress.json model and JTBD brief.", "forces-of-progress.json 模型与 JTBD 简报。"), false),
			methodNode("validate", protocol.WorkItemKindProduce, lt("Validate the opportunity", "验证机会"), lt("Test the most uncertain opportunity with a threshold and capture observed progress, not stated preference alone.", "用阈值测试最不确定的机会，记录真实进展而不只记录口头偏好。"), lt("A validation record with threshold and outcome.", "带阈值与结果的验证记录。"), false),
			methodNode("validation-review", protocol.WorkItemKindVerify, lt("Review the validation decision", "复核验证决策"), lt("Check that the evidence supports the continue, adjust, or stop choice and that the job boundary remains clear.", "检查证据是否支持继续、调整或停止，并确认 Job 边界仍清晰。"), lt("A reviewed opportunity decision.", "经过复核的机会决策。"), false),
			methodNode("deliver", protocol.WorkItemKindIntegrate, lt("Package the JTBD decision", "整理 JTBD 决策包"), lt("Publish the job, forces, opportunity, evidence, and next experiment as one reusable brief.", "把 Job、力量、机会、证据和下一项实验整理成可复用简报。"), lt("The JTBD brief and validation-record.md package.", "JTBD 简报与 validation-record.md 交付包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("interviews", "context"), dep("job", "interviews"), dep("disconfirm", "interviews"),
			dep("forces", "job"), dep("forces", "disconfirm"), dep("validate", "forces"),
			dep("validation-review", "validate"), dep("deliver", "validation-review"),
		},
	},
	{
		slashName: "business-model", title: lt("Business Model", "商业模式"),
		description:        lt("Use this when an idea needs to become a testable business rather than a promising concept: map customers, value, channels, economics, and operations, then stress-test the riskiest assumptions. It records the nine-block model, assumption tests, economic counter-checks, and a continue/adjust/stop decision.", "当一个想法需要从“听起来不错”变成可测试的生意时使用：梳理客户、价值、渠道、经济结构和运营能力，再压力测试最高风险假设。最终会留下九块商业模式、假设测试、经济反向检查，以及继续/调整/停止决策。"),
		objective:          lt("Reach a testable business model with explicit dependencies, experiment thresholds, and a continue-adjust-stop decision.", "形成包含明确依赖、实验阈值和继续/调整/停止决策的可测试商业模式。"),
		completionCriteria: []localizedWorkflowText{lt("The nine blocks are internally consistent and their critical dependencies are explicit.", "九个模块内部一致，关键依赖清晰。"), lt("The riskiest assumptions have experiments, thresholds, and next actions.", "最高风险假设都有实验、阈值和后续动作。")},
		artifactContract:   ac("business-model", "business-model.json and the assumption ledger are canonical", "show the nine blocks as a compact canvas with risk markers", artifact("business-model.json", "business model", "JSON", "Nine blocks, links, assumptions, and confidence", "value propositions", "segments", "channels", "relationships", "revenue", "resources", "activities", "partners", "costs"), artifact("business-model.md", "model narrative", "Markdown", "Readable model, rationale, and key dependencies", "canvas", "dependencies", "risks", "next tests"), artifact("assumption-test-ledger.csv", "assumption ledger", "CSV", "Assumption, experiment, threshold, result, and owner", "assumption", "risk", "test", "threshold", "result"), artifact("continue-adjust-stop.md", "decision record", "Markdown", "Continue, adjust, or stop decision with evidence", "decision", "evidence", "trigger", "next action")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("canvas", protocol.WorkItemKindProduce, lt("Map the nine blocks", "绘制九块画布"), lt("Describe the customer, value, route to market, economics, and operating capabilities.", "描述客户、价值、市场路径、经济结构和运营能力。"), lt("The business-model.json canonical canvas.", "business-model.json 权威画布。"), false),
			methodNode("consistency", protocol.WorkItemKindVerify, lt("Check consistency and dependencies", "检查一致性与依赖"), lt("Find contradictions, missing links, and assumptions that connect multiple blocks.", "发现矛盾、缺失连接和跨多个模块的假设。"), lt("A dependency and risk review.", "依赖与风险复核。"), false),
			methodNodeRole("economics-check", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Stress-test the economics", "压力测试经济结构"), lt("Challenge willingness to pay, cost drivers, channel efficiency, and the path to sustainable margin.", "挑战支付意愿、成本驱动、渠道效率和可持续利润路径。"), lt("An independent economics challenge.", "独立的经济结构挑战记录。"), false),
			methodNode("experiments", protocol.WorkItemKindProduce, lt("Design assumption tests", "设计假设测试"), lt("Prioritize the riskiest assumptions and define experiments, thresholds, and owners.", "为最高风险假设排序，定义实验、阈值和负责人。"), lt("The assumption-test-ledger.csv plan.", "assumption-test-ledger.csv 计划。"), false),
			methodNode("evidence", protocol.WorkItemKindVerify, lt("Run experiments and read evidence", "执行实验并读取证据"), lt("Run the highest-risk tests and record results against precommitted thresholds.", "执行最高风险测试，按预先承诺的阈值记录结果。"), lt("A completed assumption-test-ledger.csv with evidence.", "带证据的 assumption-test-ledger.csv。"), false),
			methodNodeRole("countertest", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Run the counter-test", "执行反向测试"), lt("Try to disprove the model with an alternative segment, pricing, or cost scenario.", "用替代客户、定价或成本情景尝试推翻模型。"), lt("A counter-test result and boundary conditions.", "反向测试结果与边界条件。"), false),
			methodNode("decision", protocol.WorkItemKindIntegrate, lt("Decide continue, adjust, or stop", "决定继续、调整或停止"), lt("Reconcile experiments and counter-tests into the next investment decision and preserve the model revision.", "把实验与反向测试汇合为下一步投入决策，并保留模型版本。"), lt("The business model and continue-adjust-stop decision package.", "商业模式与继续/调整/停止决策包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("consistency", "canvas"), dep("economics-check", "canvas"), dep("experiments", "consistency"),
			dep("experiments", "economics-check"), dep("evidence", "experiments"), dep("countertest", "experiments"),
			dep("decision", "evidence"), dep("decision", "countertest"),
		},
	},
	{
		slashName: "pyramid-brief", title: lt("Pyramid Brief", "金字塔表达"),
		description:        lt("Use this when a decision maker needs a short answer that can survive scrutiny: state the conclusion first, group the supporting arguments, trace claims to evidence, and test the strongest counterargument. It records an executive brief, argument-evidence matrix, and logic audit.", "当决策者需要一份简短但经得起追问的答案时使用：先给结论，再组织支撑论点，把论点追溯到证据，并检验最强反方。最终会留下高管简报、论点—证据矩阵和逻辑审计。"),
		objective:          lt("Produce a concise decision brief whose conclusion is supported by a coherent, auditable argument structure.", "产出一份结论明确、论证连贯且可审计的决策简报。"),
		completionCriteria: []localizedWorkflowText{lt("The answer is supported by grouped arguments without overlap or missing material support.", "结论由归类论点支撑，没有重复或重要支撑缺口。"), lt("Evidence, inference, counterarguments, and logic gaps are distinguishable.", "证据、推断、反方论点和逻辑缺口可区分。")},
		artifactContract:   ac("pyramid-brief", "argument-evidence-matrix.csv is canonical", "show conclusion at the top with expandable argument branches", artifact("executive-brief.md", "executive brief", "Markdown", "Answer-first brief for a decision maker", "answer", "arguments", "evidence", "implications", "ask"), artifact("argument-evidence-matrix.csv", "argument matrix", "CSV", "Claims, support, source, confidence, and counterargument", "claim", "parent", "evidence", "confidence", "counterargument"), artifact("logic-audit.md", "logic audit", "Markdown", "MECE, inference, and missing-support checks", "test", "finding", "severity", "repair")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("answer", protocol.WorkItemKindProduce, lt("State the answer", "先给出结论"), lt("Write the decision, recommendation, and immediate implication in one clear answer.", "用清晰的一句话写出决策、建议和直接影响。"), lt("An answer-first executive brief outline.", "先给结论的高管简报提纲。"), false),
			methodNode("arguments", protocol.WorkItemKindProduce, lt("Organize the arguments", "组织论点"), lt("Group supporting arguments by a coherent governing thought and remove overlap.", "按统一主旨组织支撑论点并消除重复。"), lt("A hierarchical argument structure.", "层次化论证结构。"), false),
			methodNode("evidence", protocol.WorkItemKindVerify, lt("Attach supporting evidence", "附上支撑证据"), lt("Trace each material claim to evidence and test whether the evidence actually supports the claim.", "让每个重要论点回溯到证据，并检查证据是否真的支持论点。"), lt("A claim-to-evidence matrix.", "结论到证据矩阵。"), false),
			methodNodeRole("counterargument", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Test the counterargument", "检验反方论点"), lt("State the strongest opposing case and identify what would weaken or overturn the answer.", "写出最强反方论点，并说明什么会削弱或推翻结论。"), lt("A counterargument register with response conditions.", "带回应条件的反方登记。"), false),
			methodNode("logic-audit", protocol.WorkItemKindVerify, lt("Audit the pyramid logic", "审计金字塔逻辑"), lt("Check grouping, inference, evidence sufficiency, and the path from arguments to answer.", "检查分组、推断、证据充分性，以及论点到结论的路径。"), lt("The logic audit with resolved gaps.", "带已解决缺口的逻辑审计。"), false),
			methodNode("deliver", protocol.WorkItemKindIntegrate, lt("Deliver the brief", "交付简报"), lt("Edit for the decision maker and preserve the evidence path and explicit ask.", "面向决策者编辑，并保留证据路径和明确请求。"), lt("The executive-brief.md delivery package.", "executive-brief.md 交付包。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("arguments", "answer"), dep("evidence", "arguments"), dep("counterargument", "arguments"),
			dep("logic-audit", "evidence"), dep("logic-audit", "counterargument"), dep("deliver", "logic-audit"),
		},
	},
	{
		slashName: "ooda-loop", title: lt("OODA Loop", "OODA 循环"),
		description:        lt("Use this in a changing situation where waiting for a perfect plan is more dangerous than learning quickly: observe a bounded window, orient the options, red-team the assumptions, act, and feed the result into the next cycle. It records the decision, observation log, and action-feedback loop.", "当环境变化快、等待完美计划的代价高于快速学习时使用：观察一个有边界的窗口，形成判断，红队挑战假设，采取行动，再把结果带回下一轮。最终会留下决策记录、观察日志和行动—反馈循环。"),
		objective:          lt("Make a timely decision while preserving the observations, assumptions, action, and feedback needed for the next cycle.", "在及时决策的同时，保留下一轮所需的观察、假设、行动和反馈。"),
		completionCriteria: []localizedWorkflowText{lt("The decision is grounded in a named observation window and orientation assumptions.", "决策建立在明确的观察窗口和定向假设上。"), lt("Action outcome and feedback determine the next cycle or explicit stop.", "行动结果和反馈决定下一轮或明确停止。")},
		artifactContract:   ac("ooda-loop", "observation-orientation-log.csv and action-feedback-log.csv are canonical", "show a circular loop with current cycle highlighted", artifact("ooda-decision.md", "decision record", "Markdown", "Current observation, orientation, decision, and action", "cycle", "observation", "orientation", "decision", "action"), artifact("observation-orientation-log.csv", "observation log", "CSV", "Time-bounded observations, assumptions, and confidence", "cycle", "window", "observation", "assumption", "confidence"), artifact("action-feedback-log.csv", "feedback log", "CSV", "Action, result, signal, and next-cycle trigger", "cycle", "action", "result", "signal", "trigger")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("observe", protocol.WorkItemKindProduce, lt("Observe the situation", "观察情势"), lt("Collect relevant signals in a named time window and separate observation from interpretation.", "在明确时间窗口收集相关信号，区分观察和解释。"), lt("A bounded observation log.", "有边界的观察记录。"), false),
			methodNode("orient", protocol.WorkItemKindProduce, lt("Orient the decision", "定向判断"), lt("Update the mental model, assumptions, options, and constraints using the observations.", "依据观察更新模型、假设、选项和约束。"), lt("An orientation note with confidence and unknowns.", "包含信心与未知项的定向笔记。"), false),
			methodNode("options", protocol.WorkItemKindProduce, lt("Choose an action path", "形成行动路径"), lt("Compare options against the current orientation and state the expected signal for each.", "依据当前定向比较行动选项，并为每项写出预期信号。"), lt("An option and trigger set.", "行动选项与触发条件集。"), false),
			methodNodeRole("red-team", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Red-team the orientation", "红队挑战定向判断"), lt("Challenge the assumptions, missing signals, and likely failure modes before committing to action.", "在行动前挑战假设、遗漏信号和可能的失败模式。"), lt("A red-team challenge note.", "红队挑战记录。"), false),
			methodNode("decide", protocol.WorkItemKindVerify, lt("Decide and set a trigger", "决策并设置触发条件"), lt("Reconcile the option path and red-team challenge, then choose an action and revisit trigger.", "把行动路径与红队意见汇合，选择行动并设置复盘触发条件。"), lt("The ooda-decision.md decision record.", "ooda-decision.md 决策记录。"), false),
			methodNode("act", protocol.WorkItemKindProduce, lt("Act within the boundary", "在边界内行动"), lt("Execute the bounded action and record the outcome without silently changing the decision scope.", "执行有边界的行动，记录结果，不悄悄改变决策范围。"), lt("A bounded action record.", "有边界的行动记录。"), false),
			methodNode("feedback", protocol.WorkItemKindIntegrate, lt("Capture feedback and route the next cycle", "记录反馈并进入下一轮"), lt("Compare the outcome with the expected signal and decide whether to loop, revise orientation, or stop.", "把结果与预期信号对照，决定循环、修正定向或停止。"), lt("The action-feedback-log.csv feedback record.", "action-feedback-log.csv 反馈记录。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("orient", "observe"), dep("options", "orient"), dep("red-team", "orient"),
			dep("decide", "options"), dep("decide", "red-team"), dep("act", "decide"), dep("feedback", "act"),
		},
	},
	{
		slashName: "ontology-model", title: lt("Ontology Operating Model", "本体论运营模型"),
		description:        lt("Use this when repeated operational work needs a shared model that connects source data to governed action: define objects and links, preserve lineage, specify permissions, and prove one real use case end to end. It records the canonical ontology, relationship and evidence tables, and action-governance rules.", "当重复性的运营工作需要一套把来源数据连接到受治理动作的共同模型时使用：定义对象和关系，保留数据血缘，规定权限，并用一个真实用例跑通全链路。最终会留下权威本体、关系与证据表，以及动作治理规则。"),
		objective:          lt("Produce a governed ontology that can explain a real operational decision from source evidence through action and writeback.", "形成一套有治理的本体论，能把真实运营决策从来源证据解释到行动和回写。"),
		completionCriteria: []localizedWorkflowText{lt("Object types, properties, links, actions, permissions, and source lineage have stable identifiers.", "对象类型、属性、关系、动作、权限和来源血缘都有稳定标识。"), lt("At least one real use case can traverse objects to a governed action and record its outcome.", "至少一个真实用例可以从对象走到受治理动作，并记录结果。")},
		artifactContract:   ac("ontology-operating-model", "ontology.yaml is canonical; CSVs are audit projections", "show object-link graph plus an action/governance side panel", artifact("ontology.yaml", "canonical ontology", "YAML", "Object types, properties, links, actions, governance, and IDs", "object types", "properties", "links", "actions", "governance"), artifact("ontology-model.md", "ontology guide", "Markdown", "Meaning, boundaries, use cases, and operating semantics", "scope", "objects", "relationships", "use case", "limitations"), artifact("object-link-matrix.csv", "relationship matrix", "CSV", "Object instances/types and their typed links", "subject", "link", "object", "cardinality", "status"), artifact("evidence-lineage.csv", "evidence lineage", "CSV", "Source records, transformations, timestamps, and confidence", "field", "source", "transformation", "observed at", "confidence"), artifact("action-governance.md", "action governance", "Markdown", "Action inputs, preconditions, permissions, side effects, and audit", "action", "preconditions", "permission", "side effects", "audit")),
		nodes: []builtinWorkflowNodeDefinition{
			methodNode("scope", protocol.WorkItemKindProduce, lt("Set the ontology boundary", "设定本体论边界"), lt("Choose a real operational use case, source systems, time horizon, and ownership boundary.", "选择真实运营用例、来源系统、时间范围和责任边界。"), lt("A bounded ontology scope and use-case statement.", "有边界的本体论范围与用例声明。"), false),
			methodNode("objects", protocol.WorkItemKindProduce, lt("Define objects and properties", "定义对象与属性"), lt("Define stable object types, identifiers, properties, quality rules, and source mappings.", "定义稳定对象类型、标识、属性、质量规则和来源映射。"), lt("The ontology.yaml object and property model.", "ontology.yaml 对象与属性模型。"), false),
			methodNode("links", protocol.WorkItemKindProduce, lt("Define links and evidence lineage", "定义关系与证据血缘"), lt("Describe typed relationships, cardinality, provenance, transformations, and confidence.", "描述类型化关系、基数、来源、转换和置信度。"), lt("The object-link-matrix.csv and evidence-lineage.csv audit projections.", "object-link-matrix.csv 与 evidence-lineage.csv 审计投影。"), false),
			methodNode("source-audit", protocol.WorkItemKindVerify, lt("Audit source fidelity", "审计来源一致性"), lt("Check whether object properties and links can be traced to source records without semantic drift.", "检查对象属性和关系能否回溯到来源记录，避免语义漂移。"), lt("An evidence-lineage audit with unresolved gaps.", "带未决缺口的证据血缘审计。"), false),
			methodNode("actions", protocol.WorkItemKindProduce, lt("Define governed actions", "定义受治理动作"), lt("Specify action inputs, preconditions, permissions, side effects, and durable audit records.", "规定动作输入、前置条件、权限、副作用和持久审计记录。"), lt("The action-governance.md contract.", "action-governance.md 契约。"), false),
			methodNodeRole("policy-review", protocol.WorkItemKindReview, protocol.WorkGraphWorkflowNodeCollaboration, lt("Review action governance", "复核动作治理"), lt("Independently challenge permissions, segregation of duties, failure handling, and writeback consequences.", "独立挑战权限、职责分离、失败处理和回写后果。"), lt("A governance review with resolved blockers.", "带已解决阻塞项的治理复核。"), false),
			methodNode("operate", protocol.WorkItemKindIntegrate, lt("Prove the operating use case", "验证运营用例"), lt("Walk a real case from source evidence through object state, action, and outcome writeback.", "用真实案例走通从来源证据、对象状态到动作和结果回写的链路。"), lt("The ontology-model.md operating guide and a reproducible use-case record.", "ontology-model.md 运营指南与可复现用例记录。"), true),
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			dep("objects", "scope"), dep("links", "objects"), dep("source-audit", "objects"),
			dep("actions", "links"), dep("actions", "source-audit"), dep("policy-review", "actions"), dep("operate", "policy-review"),
		},
	},
}

func lt(english, chinese string) localizedWorkflowText {
	return localizedWorkflowText{english: english, chinese: chinese}
}

func methodNode(key string, kind protocol.WorkItemKind, subject, objective, deliverable localizedWorkflowText, terminal bool) builtinWorkflowNodeDefinition {
	return methodNodeRole(key, kind, protocol.WorkGraphWorkflowNodeKey, subject, objective, deliverable, terminal)
}

func methodNodeRole(key string, kind protocol.WorkItemKind, role protocol.WorkGraphWorkflowNodeRole, subject, objective, deliverable localizedWorkflowText, terminal bool) builtinWorkflowNodeDefinition {
	return builtinWorkflowNodeDefinition{
		logicalKey: key, role: role, kind: kind,
		subject: subject, objective: objective, deliverable: deliverable,
		acceptanceCriteria: []localizedWorkflowText{lt("The stage output is explicit, reviewable, and sufficient for the next stage.", "阶段产物明确、可复核，并足以支持下一阶段。")},
		required:           true, terminal: terminal,
	}
}

func dep(logicalKey, dependsOn string) protocol.WorkGraphWorkflowDependency {
	return protocol.WorkGraphWorkflowDependency{LogicalKey: logicalKey, DependsOnLogicalKey: dependsOn, Kind: protocol.WorkDependencyHard}
}

func chain(keys ...string) []protocol.WorkGraphWorkflowDependency {
	result := make([]protocol.WorkGraphWorkflowDependency, 0, len(keys)-1)
	for index := 1; index < len(keys); index++ {
		result = append(result, protocol.WorkGraphWorkflowDependency{
			LogicalKey: keys[index], DependsOnLogicalKey: keys[index-1], Kind: protocol.WorkDependencyHard,
		})
	}
	return result
}

func artifact(name, kind, format, purpose string, sections ...string) protocol.WorkGraphArtifactSpec {
	return protocol.WorkGraphArtifactSpec{Name: name, Kind: kind, Format: format, Purpose: purpose, RequiredSections: sections}
}

func ac(profile, sourceOfTruth, renderHint string, primary protocol.WorkGraphArtifactSpec, supporting ...protocol.WorkGraphArtifactSpec) *protocol.WorkGraphArtifactContract {
	return &protocol.WorkGraphArtifactContract{Profile: profile, SourceOfTruth: sourceOfTruth, RenderHint: renderHint, Primary: primary, Supporting: supporting}
}
