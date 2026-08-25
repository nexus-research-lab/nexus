// INPUT: Nexus 发布内置的通用责任拓扑与目标展示语言。
// OUTPUT: 可直接进入工作图目录、Slash catalog 和 runtime 展开的只读系统模板。
// POS: 内置 WorkGraph 模板的唯一语义定义；不写数据库，也不携带任何 Execution 运行事实。
package workgraphworkflow

import (
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const builtinWorkflowIDPrefix = "builtin-workgraph-"

var builtinWorkflowPublishedAt = time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC)

type localizedWorkflowText struct {
	english string
	chinese string
}

func (text localizedWorkflowText) localized(locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") && text.chinese != "" {
		return text.chinese
	}
	return text.english
}

type builtinWorkflowNodeDefinition struct {
	logicalKey         string
	role               protocol.WorkGraphWorkflowNodeRole
	kind               protocol.WorkItemKind
	subject            localizedWorkflowText
	objective          localizedWorkflowText
	deliverable        localizedWorkflowText
	acceptanceCriteria []localizedWorkflowText
	required           bool
	terminal           bool
	parentLogicalKey   string
}

type builtinWorkflowDefinition struct {
	slashName          string
	title              localizedWorkflowText
	description        localizedWorkflowText
	objective          localizedWorkflowText
	completionCriteria []localizedWorkflowText
	nodes              []builtinWorkflowNodeDefinition
	dependencies       []protocol.WorkGraphWorkflowDependency
}

var builtinWorkflowDefinitions = []builtinWorkflowDefinition{
	{
		slashName: "deep-research",
		title:     localizedWorkflowText{english: "Deep Research", chinese: "深度研究"},
		description: localizedWorkflowText{
			english: "Decompose a question, plan and run independent evidence tracks, evaluate sufficiency, adapt weak search strategies through explicit research iterations, verify the synthesis, and deliver a cited answer.",
			chinese: "拆分问题并规划独立证据线，评估证据充分性；证据薄弱时显式调整搜索策略并多轮补充研究，随后核验综合结论并交付带引用的答案。",
		},
		objective: localizedWorkflowText{
			english: "Answer a consequential research question with broad, traceable evidence, explicit uncertainty, and an independently checked final synthesis.",
			chinese: "用广泛且可追溯的证据回答重要研究问题，明确不确定性，并对最终综合结论进行独立核验。",
		},
		completionCriteria: []localizedWorkflowText{
			{english: "Material claims are traceable to credible sources.", chinese: "重要结论均可追溯到可信来源。"},
			{english: "Conflicting evidence and meaningful uncertainty are explicit.", chinese: "冲突证据与重要不确定性均被明确说明。"},
			{english: "The evidence-sufficiency gate passed after every necessary strategy-adjustment and targeted-research iteration.", chinese: "证据充分性 Gate 已在所有必要的策略调整与定向补充研究迭代后通过。"},
			{english: "The final answer directly resolves the framed question.", chinese: "最终答案直接回应已界定的研究问题。"},
		},
		nodes: []builtinWorkflowNodeDefinition{
			{
				logicalKey: "frame", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce,
				subject:            localizedWorkflowText{english: "Frame the research", chinese: "界定研究问题"},
				objective:          localizedWorkflowText{english: "Define the decision question, scope, constraints, source standards, and what would count as a useful answer.", chinese: "明确决策问题、范围、约束、来源标准，以及什么样的答案才真正有用。"},
				deliverable:        localizedWorkflowText{english: "A research brief with subquestions and evidence requirements.", chinese: "包含子问题与证据要求的研究简报。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "The brief states the core question and boundaries.", chinese: "简报明确核心问题与研究边界。"}, {english: "Source quality and recency requirements are explicit.", chinese: "来源质量与时效要求清晰。"}},
				required:           true,
			},
			{
				logicalKey: "research-strategy-1", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce,
				subject:            localizedWorkflowText{english: "Iteration 1 · Design the broad research strategy", chinese: "第一轮 · 设计广泛研究策略"},
				objective:          localizedWorkflowText{english: "Decompose the framed question into independently researchable parts and define source classes, search paths, evidence-quality rules, iteration limits, and stop conditions for each part.", chinese: "把已界定的问题拆成可独立研究的部分，并为每部分定义来源类型、搜索路径、证据质量规则、迭代上限与停止条件。"},
				deliverable:        localizedWorkflowText{english: "A partitioned research plan with search strategies and sufficiency criteria per subquestion.", chinese: "按子问题拆分、包含搜索策略与充分性标准的研究计划。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "Every material subquestion has an explicit collection strategy.", chinese: "每个重要子问题都有明确的收集策略。"}, {english: "Evidence-quality, iteration, and stop rules are testable.", chinese: "证据质量、迭代与停止规则均可验证。"}},
				required:           true,
			},
			{
				logicalKey: "authoritative-evidence-1", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindProduce,
				subject:            localizedWorkflowText{english: "Iteration 1 · Authoritative evidence", chinese: "第一轮 · 权威证据线"},
				objective:          localizedWorkflowText{english: "Collect primary, official, or otherwise authoritative evidence for the framed subquestions.", chinese: "围绕已界定的子问题收集一手、官方或其他权威证据。"},
				deliverable:        localizedWorkflowText{english: "Structured source notes with claim-to-source links.", chinese: "带有结论到来源映射的结构化资料笔记。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "Each material claim has a traceable source.", chinese: "每项重要结论都有可追溯来源。"}, {english: "Source limitations are recorded.", chinese: "来源局限已被记录。"}},
				required:           true,
			},
			{
				logicalKey: "contrasting-evidence-1", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindProduce,
				subject:            localizedWorkflowText{english: "Iteration 1 · Contrasting evidence", chinese: "第一轮 · 对照证据线"},
				objective:          localizedWorkflowText{english: "Seek independent perspectives, counterexamples, and evidence that could change the leading interpretation.", chinese: "寻找独立观点、反例，以及可能改变主导解释的证据。"},
				deliverable:        localizedWorkflowText{english: "A contrasting evidence set with disagreements and gaps.", chinese: "标注分歧与缺口的对照证据集。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "Meaningful counterevidence was actively sought.", chinese: "已主动寻找有意义的反证。"}, {english: "Disagreements are represented fairly.", chinese: "不同观点得到公平呈现。"}},
				required:           true,
			},
			{
				logicalKey: "evidence-evaluation-1", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindVerify,
				subject:            localizedWorkflowText{english: "Iteration 1 · Evaluate evidence quality and gaps", chinese: "第一轮 · 评估证据质量与缺口"},
				objective:          localizedWorkflowText{english: "Evaluate relevance, credibility, independence, recency, contradiction, and coverage for every subquestion, then produce an explicit sufficient/insufficient verdict. Acceptance confirms the evaluation is sound, not that evidence is sufficient. When insufficient, append Iteration N+1 to this same Execution and WorkGraph: diagnose the gaps, materially adjust the strategy, collect targeted authoritative and contrasting evidence in parallel, and evaluate again. Replace synthesis' prerequisite with the latest evaluation and repeat until sufficient or a declared iteration limit or stop condition is reached; never synthesize merely to finish.", chinese: "逐个子问题评估相关性、可信度、独立性、时效性、冲突与覆盖度，并给出明确的充分/不足结论。验收代表评估本身可靠，不代表证据已经充分。若证据不足，应在同一 Execution、同一工作图中追加第 N+1 轮：诊断缺口、实质调整策略、并行定向收集权威证据与对照证据，然后再次评估；同时把综合节点的前置依赖替换为最新一轮评估。如此重复，直到证据充分，或触发已声明的迭代上限/停止条件；不能为了结束而直接综合。"},
				deliverable:        localizedWorkflowText{english: "A cumulative evidence matrix with a sufficiency verdict, prioritized gaps, and the next-iteration decision when needed.", chinese: "包含充分性结论、优先缺口，以及必要时下一轮决策的累计证据矩阵。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "Every framed subquestion has an explicit evidence-quality and coverage decision.", chinese: "每个已界定子问题都有明确的证据质量与覆盖判断。"}, {english: "An insufficient verdict defines specific prioritized gaps and a materially changed next strategy instead of repeating the same search.", chinese: "证据不足时明确具体、有优先级的缺口与实质变化的下一轮策略，而不是重复相同搜索。"}},
				required:           true,
			},
			{
				logicalKey: "synthesize", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate,
				subject:            localizedWorkflowText{english: "Synthesize findings", chinese: "综合研究发现"},
				objective:          localizedWorkflowText{english: "After coverage is accepted, reconcile the evidence tracks into findings, confidence levels, open questions, and implications.", chinese: "覆盖审计通过后，把多条证据线整合为研究发现、置信程度、开放问题与影响判断。"},
				deliverable:        localizedWorkflowText{english: "An evidence synthesis with explicit uncertainty.", chinese: "明确表达不确定性的证据综合稿。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "The synthesis distinguishes evidence from inference.", chinese: "综合稿区分证据与推断。"}, {english: "Conflicts and gaps remain visible.", chinese: "冲突与缺口保持可见。"}},
				required:           true,
			},
			{
				logicalKey: "verify", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindVerify,
				subject:            localizedWorkflowText{english: "Verify claims and citations", chinese: "核验结论与引用"},
				objective:          localizedWorkflowText{english: "Independently check pivotal claims, calculations, citations, and whether the synthesis overstates the evidence.", chinese: "独立检查关键结论、计算、引用，以及综合稿是否夸大证据。"},
				deliverable:        localizedWorkflowText{english: "A verification record with corrections and unresolved risks.", chinese: "包含修正项与未解决风险的核验记录。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "Pivotal claims were checked against their sources.", chinese: "关键结论已回查原始来源。"}, {english: "Corrections are actionable and specific.", chinese: "修正意见具体且可执行。"}},
				required:           true,
			},
			{
				logicalKey: "report", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate,
				subject:            localizedWorkflowText{english: "Deliver the research report", chinese: "交付研究报告"},
				objective:          localizedWorkflowText{english: "Incorporate verification and present a direct, cited answer with implications, limitations, and next steps.", chinese: "吸收核验结果，给出直接、带引用的答案，并说明影响、局限与后续行动。"},
				deliverable:        localizedWorkflowText{english: "A decision-ready cited research report.", chinese: "可直接支持决策的带引用研究报告。"},
				acceptanceCriteria: []localizedWorkflowText{{english: "The report answers the framed question.", chinese: "报告直接回答已界定的问题。"}, {english: "Citations, uncertainty, and limitations are clear.", chinese: "引用、不确定性与局限表达清楚。"}},
				required:           true, terminal: true,
			},
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			{LogicalKey: "research-strategy-1", DependsOnLogicalKey: "frame", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "authoritative-evidence-1", DependsOnLogicalKey: "research-strategy-1", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "contrasting-evidence-1", DependsOnLogicalKey: "research-strategy-1", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "evidence-evaluation-1", DependsOnLogicalKey: "authoritative-evidence-1", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "evidence-evaluation-1", DependsOnLogicalKey: "contrasting-evidence-1", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "synthesize", DependsOnLogicalKey: "evidence-evaluation-1", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "verify", DependsOnLogicalKey: "synthesize", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "report", DependsOnLogicalKey: "verify", Kind: protocol.WorkDependencyHard},
		},
	},
	{
		slashName:   "build-ship",
		title:       localizedWorkflowText{english: "Build & Ship", chinese: "构建与交付"},
		description: localizedWorkflowText{english: "Turn a scoped request into a designed and implemented change, then adaptively remediate classified validation or review blockers until it is handoff-ready.", chinese: "把已界定的需求推进为设计与实现完成的变更，并针对验证或评审 blocker 分类迭代修复，直到可安全交接。"},
		objective:   localizedWorkflowText{english: "Deliver a working change that satisfies an explicit acceptance contract, passes independent review and validation, and is ready to hand off.", chinese: "交付满足明确验收合同、通过独立评审与验证，并可直接交接的可用变更。"},
		completionCriteria: []localizedWorkflowText{
			{english: "The delivered change satisfies the agreed acceptance criteria.", chinese: "交付变更满足约定的验收条件。"},
			{english: "The latest quality gate passed after every necessary remediation, revalidation, and rereview iteration.", chinese: "在所有必要的修复、重新验证与重新评审迭代后，最新质量 Gate 已通过。"},
			{english: "Operational or user-facing handoff material is complete.", chinese: "运维或用户侧交接材料完整。"},
		},
		nodes: []builtinWorkflowNodeDefinition{
			{logicalKey: "scope", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Scope the change", chinese: "界定变更范围"}, objective: localizedWorkflowText{english: "Clarify the outcome, constraints, affected surfaces, risks, and acceptance criteria.", chinese: "明确结果、约束、受影响范围、风险与验收条件。"}, deliverable: localizedWorkflowText{english: "An implementation-ready change brief.", chinese: "可直接进入实现的变更简报。"}, acceptanceCriteria: []localizedWorkflowText{{english: "In-scope and out-of-scope boundaries are explicit.", chinese: "范围内与范围外边界清晰。"}, {english: "Acceptance criteria are testable.", chinese: "验收条件可验证。"}}, required: true},
			{logicalKey: "design", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Design the solution", chinese: "设计解决方案"}, objective: localizedWorkflowText{english: "Choose a simple solution and identify interfaces, migration needs, failure modes, and the verification plan.", chinese: "选择简洁方案，并识别接口、迁移需求、失败模式与验证计划。"}, deliverable: localizedWorkflowText{english: "A concrete solution design and change plan.", chinese: "具体的方案设计与变更计划。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The design covers affected boundaries and failure modes.", chinese: "设计覆盖受影响边界与失败模式。"}, {english: "The verification approach is defined before implementation.", chinese: "实现前已定义验证方法。"}}, required: true},
			{logicalKey: "implement", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Implement the change", chinese: "实现变更"}, objective: localizedWorkflowText{english: "Produce the requested change according to the approved scope and design.", chinese: "按照已确认的范围与设计完成所需变更。"}, deliverable: localizedWorkflowText{english: "A complete working implementation with necessary supporting material.", chinese: "完整可用的实现及必要配套材料。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The implementation covers every in-scope requirement.", chinese: "实现覆盖所有范围内需求。"}, {english: "Relevant documentation or configuration is updated.", chinese: "相关文档或配置已同步更新。"}}, required: true},
			{logicalKey: "validate", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindVerify, subject: localizedWorkflowText{english: "Validate behavior", chinese: "验证行为"}, objective: localizedWorkflowText{english: "Test the change against acceptance criteria, edge cases, and regression risks.", chinese: "依据验收条件、边界场景与回归风险测试变更。"}, deliverable: localizedWorkflowText{english: "A validation record with reproducible evidence.", chinese: "包含可复现证据的验证记录。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Acceptance paths and material edge cases were exercised.", chinese: "已覆盖验收路径与重要边界场景。"}, {english: "Failures identify exact corrective work.", chinese: "失败项明确指出具体修正工作。"}}, required: true},
			{logicalKey: "review", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindReview, subject: localizedWorkflowText{english: "Review the change", chinese: "独立评审变更"}, objective: localizedWorkflowText{english: "Independently review correctness, maintainability, safety, and hidden scope or regression risks.", chinese: "独立评审正确性、可维护性、安全性，以及隐藏范围或回归风险。"}, deliverable: localizedWorkflowText{english: "A review decision with prioritized findings.", chinese: "带优先级问题清单的评审结论。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Blocking findings are distinguished from optional improvements.", chinese: "阻塞问题与可选改进已区分。"}, {english: "Findings are specific and evidence-based.", chinese: "问题具体且有证据支持。"}}, required: true},
			{logicalKey: "quality-gate-1", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindVerify, subject: localizedWorkflowText{english: "Iteration 1 · Decide release readiness", chinese: "第 1 轮 · 判定交付就绪度"}, objective: localizedWorkflowText{english: "Evaluate validation and independent review together and produce an explicit sufficient/insufficient release-readiness verdict. Acceptance confirms the gate assessment is sound. If insufficient, classify each blocker as scope, design, implementation, test, documentation, or external dependency work, then append only the necessary Iteration N+1 remediation nodes to this same Execution and WorkGraph. Material changes must converge through parallel revalidation and independent rereview before a new quality gate. Move delivery's prerequisite to the latest gate and repeat until sufficient or a declared stop or blocked condition is reached.", chinese: "综合验证与独立评审结果，给出明确的交付就绪度充分/不足结论。验收代表 Gate 判断可靠。若不足，先把 blocker 分类为范围、设计、实现、测试、文档或外部依赖问题，再在同一 Execution、同一工作图中只追加第 N+1 轮必要的修复节点；实质变更必须经过并行重新验证与独立重新评审后汇合到新的质量 Gate。同时把交付节点的前置依赖移到最新 Gate，直到充分或触发已声明的停止/阻塞条件。"}, deliverable: localizedWorkflowText{english: "A release-readiness decision with classified blockers and a targeted next remediation graph when needed.", chinese: "包含已分类 blocker 及必要时定向下一轮修复图的交付就绪度结论。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Every validation and review blocker is resolved or explicitly routed to the correct remediation boundary.", chinese: "每个验证与评审 blocker 均已解决，或明确路由到正确的修复边界。"}, {english: "The verdict is supported by reproducible validation and specific review evidence.", chinese: "结论由可复现验证与具体评审证据支持。"}}, required: true},
			{logicalKey: "deliver", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate, subject: localizedWorkflowText{english: "Integrate and hand off", chinese: "整合并交付"}, objective: localizedWorkflowText{english: "Resolve blocking findings, consolidate validation evidence, and prepare the final change for use or release.", chinese: "解决阻塞问题，汇总验证证据，并把最终变更准备为可使用或可发布状态。"}, deliverable: localizedWorkflowText{english: "The final change with evidence, release notes, and handoff instructions.", chinese: "最终变更、验证证据、发布说明与交接指引。"}, acceptanceCriteria: []localizedWorkflowText{{english: "No review or validation blocker remains.", chinese: "评审与验证均无遗留阻塞。"}, {english: "A recipient can use or release the result safely.", chinese: "接收方可以安全使用或发布结果。"}}, required: true, terminal: true},
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			{LogicalKey: "design", DependsOnLogicalKey: "scope", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "implement", DependsOnLogicalKey: "design", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "validate", DependsOnLogicalKey: "implement", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "review", DependsOnLogicalKey: "implement", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "quality-gate-1", DependsOnLogicalKey: "validate", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "quality-gate-1", DependsOnLogicalKey: "review", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "deliver", DependsOnLogicalKey: "quality-gate-1", Kind: protocol.WorkDependencyHard},
		},
	},
	{
		slashName:          "decision-brief",
		title:              localizedWorkflowText{english: "Decision Brief", chinese: "决策简报"},
		description:        localizedWorkflowText{english: "Frame a decision, develop evidence and options in parallel, then adapt the evidence, criteria, options, or experiment path until the challenged recommendation is defensible.", chinese: "界定决策后并行发展证据与方案，再按挑战结果调整证据、标准、方案或实验路径，直到建议可辩护。"},
		objective:          localizedWorkflowText{english: "Make a defensible decision by comparing viable options against explicit criteria, testing assumptions, and documenting the recommendation and its conditions.", chinese: "用明确标准比较可行方案、检验关键假设，并记录建议及其适用条件，形成可辩护的决策。"},
		completionCriteria: []localizedWorkflowText{{english: "Viable options are compared against explicit criteria.", chinese: "可行方案已依据明确标准比较。"}, {english: "The latest challenge gate passed after every necessary evidence, option, and evaluation revision iteration.", chinese: "在所有必要的证据、方案与评估修订迭代后，最新挑战 Gate 已通过。"}, {english: "Material assumptions, risks, and reversibility are visible.", chinese: "重要假设、风险与可逆性清晰可见。"}, {english: "The recommendation includes conditions and next actions.", chinese: "建议包含适用条件与下一步行动。"}},
		nodes: []builtinWorkflowNodeDefinition{
			{logicalKey: "frame", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Frame the decision", chinese: "界定决策"}, objective: localizedWorkflowText{english: "Define the decision, stakeholders, constraints, criteria, deadline, and reversibility.", chinese: "明确决策事项、相关方、约束、标准、时限与可逆性。"}, deliverable: localizedWorkflowText{english: "A decision frame and evaluation rubric.", chinese: "决策框架与评价标准。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The decision and decision owner are explicit.", chinese: "决策事项与决策责任人明确。"}, {english: "Criteria are prioritized or weighted where useful.", chinese: "必要时已对评价标准排序或赋权。"}}, required: true},
			{logicalKey: "evidence", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Build the evidence base", chinese: "建立证据基础"}, objective: localizedWorkflowText{english: "Collect the facts, constraints, benchmarks, and uncertainty that should shape the decision.", chinese: "收集影响决策的事实、约束、基准与不确定性。"}, deliverable: localizedWorkflowText{english: "A concise evidence pack with sources and confidence notes.", chinese: "包含来源与置信说明的精炼证据包。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Evidence is relevant to the evaluation criteria.", chinese: "证据与评价标准相关。"}, {english: "Unknowns and weak evidence are marked.", chinese: "未知项与薄弱证据已标注。"}}, required: true},
			{logicalKey: "options", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Develop viable options", chinese: "发展可行方案"}, objective: localizedWorkflowText{english: "Create genuinely distinct options, including the status quo when relevant, with costs and prerequisites.", chinese: "形成真正不同的备选方案，必要时包含维持现状，并说明成本与前置条件。"}, deliverable: localizedWorkflowText{english: "A set of viable option profiles.", chinese: "一组可行方案画像。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Options are materially distinct and feasible.", chinese: "方案具有实质差异且可行。"}, {english: "Costs, dependencies, and constraints are explicit.", chinese: "成本、依赖与约束清晰。"}}, required: true},
			{logicalKey: "evaluate", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate, subject: localizedWorkflowText{english: "Evaluate tradeoffs", chinese: "评估权衡"}, objective: localizedWorkflowText{english: "Compare the options against the rubric using the evidence base and make tradeoffs explicit.", chinese: "利用证据基础按评价标准比较方案，并明确展示权衡。"}, deliverable: localizedWorkflowText{english: "A transparent option comparison and provisional conclusion.", chinese: "透明的方案对比与初步结论。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Every option is evaluated consistently.", chinese: "所有方案均以一致标准评估。"}, {english: "The decisive tradeoffs are explicit.", chinese: "决定性权衡清晰可见。"}}, required: true},
			{logicalKey: "challenge", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindReview, subject: localizedWorkflowText{english: "Iteration 1 · Challenge the analysis", chinese: "第 1 轮 · 挑战分析"}, objective: localizedWorkflowText{english: "Red-team assumptions, missing options, downside scenarios, stakeholder impacts, and signs the conclusion should change, then produce an explicit sufficient/insufficient decision-quality verdict. Acceptance confirms the challenge is sound. If insufficient, classify gaps as missing evidence, flawed criteria, missing options, or decision-critical uncertainty that needs an experiment. Append only the necessary Iteration N+1 branches to this same Execution and WorkGraph, run independent branches in parallel, then re-evaluate and challenge again. Move recommendation's prerequisite to the latest challenge. Stop with a robust recommendation, an explicit conditional/deferred decision, or a bounded experiment—not endless collection.", chinese: "反向审视假设、遗漏方案、下行情景、相关方影响及应改变结论的信号，并给出明确的决策质量充分/不足结论。验收代表挑战本身可靠。若不足，先区分缺少证据、评价标准有误、遗漏方案，或需要实验才能降低的关键不确定性；只在同一 Execution、同一工作图中追加第 N+1 轮必要分支，独立分支可并行，随后重新评估并再次挑战，同时把建议节点的前置依赖移到最新挑战。最终应得到稳健建议、明确的有条件/延后决策或有边界的实验，而不是无限收集。"}, deliverable: localizedWorkflowText{english: "A challenge verdict with classified decision gaps, required corrections or conditions, and a targeted next-iteration graph when needed.", chinese: "包含已分类决策缺口、必需修正或条件，以及必要时定向下一轮图的挑战结论。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The strongest counterargument is represented fairly.", chinese: "最强反方论点得到公平呈现。"}, {english: "Material risks are mitigated, explicitly accepted, or routed to the correct evidence, criteria, option, or experiment branch.", chinese: "重要风险已缓解、明确接受，或正确路由到证据、标准、方案或实验分支。"}}, required: true},
			{logicalKey: "recommend", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate, subject: localizedWorkflowText{english: "Deliver the recommendation", chinese: "交付决策建议"}, objective: localizedWorkflowText{english: "Incorporate the challenge review and state the recommendation, rationale, conditions, owner, and next actions.", chinese: "吸收挑战性评审，明确建议、理由、适用条件、责任人与下一步行动。"}, deliverable: localizedWorkflowText{english: "A decision-ready brief and action record.", chinese: "可直接决策的简报与行动记录。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The recommendation follows from the stated criteria and evidence.", chinese: "建议与既定标准和证据一致。"}, {english: "Conditions, triggers to revisit, and next actions are explicit.", chinese: "适用条件、重审触发器与下一步行动明确。"}}, required: true, terminal: true},
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			{LogicalKey: "evidence", DependsOnLogicalKey: "frame", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "options", DependsOnLogicalKey: "frame", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "evaluate", DependsOnLogicalKey: "evidence", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "evaluate", DependsOnLogicalKey: "options", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "challenge", DependsOnLogicalKey: "evaluate", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "recommend", DependsOnLogicalKey: "challenge", Kind: protocol.WorkDependencyHard},
		},
	},
	{
		slashName:          "review-improve",
		title:              localizedWorkflowText{english: "Review & Improve", chinese: "评审与改进"},
		description:        localizedWorkflowText{english: "Audit an artifact from independent quality and experience perspectives, then choose targeted revision, reaudit, or rebaseline iterations until improvement is verified.", chinese: "从质量与体验两个独立视角审计成果，再按失败类型选择定向修订、重审计或重建基线，直到改进通过验证。"},
		objective:          localizedWorkflowText{english: "Improve an existing artifact against an explicit rubric through independent audits, prioritized revision, and regression-aware verification.", chinese: "依据明确标准，通过独立审计、优先级修订与关注回归的验证，改进现有成果。"},
		completionCriteria: []localizedWorkflowText{{english: "The revised artifact addresses all blocking findings.", chinese: "修订成果解决了所有阻塞问题。"}, {english: "The result is measurably better against the agreed rubric.", chinese: "结果依据约定标准可衡量地变得更好。"}, {english: "The latest verification passed after every necessary diagnosis and revision iteration, with no material regression.", chinese: "在所有必要的诊断与修订迭代后，最新验证已通过且无重要回归。"}},
		nodes: []builtinWorkflowNodeDefinition{
			{logicalKey: "baseline", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Establish the baseline", chinese: "建立评审基线"}, objective: localizedWorkflowText{english: "Clarify the artifact, intended audience or behavior, constraints, and the rubric for a successful improvement.", chinese: "明确评审对象、预期受众或行为、约束，以及成功改进的评价标准。"}, deliverable: localizedWorkflowText{english: "A baseline snapshot and prioritized review rubric.", chinese: "基线快照与有优先级的评审标准。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The current artifact and desired outcome are unambiguous.", chinese: "当前成果与期望结果明确无歧义。"}, {english: "The rubric distinguishes blockers from preferences.", chinese: "评价标准区分阻塞问题与偏好。"}}, required: true},
			{logicalKey: "quality-audit", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindReview, subject: localizedWorkflowText{english: "Audit quality", chinese: "审计内在质量"}, objective: localizedWorkflowText{english: "Review correctness, completeness, consistency, maintainability, and risk against the baseline rubric.", chinese: "依据基线标准评审正确性、完整性、一致性、可维护性与风险。"}, deliverable: localizedWorkflowText{english: "A quality audit with evidence and severity.", chinese: "包含证据与严重度的质量审计。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Findings cite exact evidence or locations.", chinese: "问题引用具体证据或位置。"}, {english: "Severity reflects user or operational impact.", chinese: "严重度反映用户或运行影响。"}}, required: true},
			{logicalKey: "experience-audit", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindReview, subject: localizedWorkflowText{english: "Audit experience and fit", chinese: "审计体验与适配"}, objective: localizedWorkflowText{english: "Review clarity, usability, accessibility, audience fit, and whether the artifact solves the intended problem.", chinese: "评审清晰度、可用性、可访问性、受众适配，以及成果是否真正解决目标问题。"}, deliverable: localizedWorkflowText{english: "An experience audit with observed friction and missed needs.", chinese: "包含实际摩擦点与遗漏需求的体验审计。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Findings are tied to audience needs or observable behavior.", chinese: "问题与受众需求或可观察行为相关。"}, {english: "Subjective preferences are labeled as such.", chinese: "主观偏好已明确标注。"}}, required: true},
			{logicalKey: "prioritize", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate, subject: localizedWorkflowText{english: "Prioritize improvements", chinese: "确定改进优先级"}, objective: localizedWorkflowText{english: "Reconcile both audits into a bounded revision plan ordered by impact, urgency, effort, and dependency.", chinese: "把两份审计整合为有边界的修订计划，并按影响、紧迫度、成本与依赖排序。"}, deliverable: localizedWorkflowText{english: "A prioritized revision plan with acceptance checks.", chinese: "带验收检查项的优先级修订计划。"}, acceptanceCriteria: []localizedWorkflowText{{english: "All blocking findings have an owner action.", chinese: "所有阻塞问题都有对应处理动作。"}, {english: "Deferred findings include a rationale.", chinese: "延后问题包含明确理由。"}}, required: true},
			{logicalKey: "revise", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindProduce, subject: localizedWorkflowText{english: "Revise the artifact", chinese: "修订成果"}, objective: localizedWorkflowText{english: "Apply the prioritized changes while preserving valid behavior and intentional constraints.", chinese: "实施优先级改进，同时保留正确行为与有意约束。"}, deliverable: localizedWorkflowText{english: "A revised artifact with a concise change record.", chinese: "修订后的成果与精炼变更记录。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Every blocking action is implemented or explicitly resolved.", chinese: "每个阻塞动作都已实施或明确解决。"}, {english: "The change record maps revisions to findings.", chinese: "变更记录可映射到对应问题。"}}, required: true},
			{logicalKey: "verify", role: protocol.WorkGraphWorkflowNodeCollaboration, kind: protocol.WorkItemKindVerify, subject: localizedWorkflowText{english: "Iteration 1 · Verify the improvement", chinese: "第 1 轮 · 验证改进结果"}, objective: localizedWorkflowText{english: "Independently re-evaluate the revised artifact and produce an explicit sufficient/insufficient verdict, classifying failures as unresolved findings, regressions, no measurable improvement, or an invalid baseline/rubric. Acceptance confirms the verification is sound. If insufficient, append only the necessary Iteration N+1 nodes to this same Execution and WorkGraph: narrow failures get diagnosis, targeted revision, affected-check verification, and regression checks; broad changes may trigger renewed quality or experience audits; an invalid rubric returns to baseline and prioritization. Move delivery's prerequisite to the latest verification and repeat until sufficient or a declared stop condition is reached.", chinese: "独立重新评估修订成果并给出明确的充分/不足结论，把失败分类为问题未关闭、引入回归、没有可衡量改善，或基线/评价标准无效。验收代表验证本身可靠。若不足，只在同一 Execution、同一工作图中追加第 N+1 轮必要节点：窄问题进入诊断、定向修订、受影响检查与回归检查；大范围变更可重新触发质量或体验审计；评价标准无效则回到基线与优先级。同步把交付节点的前置依赖移到最新验证，直到充分或触发已声明的停止条件。"}, deliverable: localizedWorkflowText{english: "A verification verdict with classified failures, before-and-after evidence, and a targeted next-iteration graph when needed.", chinese: "包含已分类失败、前后对比证据及必要时定向下一轮图的验证结论。"}, acceptanceCriteria: []localizedWorkflowText{{english: "Blocking findings are closed or explicitly routed to the correct revision, reaudit, or rebaseline boundary.", chinese: "阻塞问题均已关闭，或明确路由到正确的修订、重审计或重建基线边界。"}, {english: "The result improves the baseline without material regression.", chinese: "结果相较基线有所改进且无重要回归。"}}, required: true},
			{logicalKey: "deliver", role: protocol.WorkGraphWorkflowNodeKey, kind: protocol.WorkItemKindIntegrate, subject: localizedWorkflowText{english: "Deliver the improved artifact", chinese: "交付改进成果"}, objective: localizedWorkflowText{english: "Package the accepted revision, verification evidence, resolved findings, and any explicitly deferred non-blocking work for the recipient.", chinese: "面向接收方整合已通过的修订成果、验证证据、已解决问题，以及明确延后的非阻塞工作。"}, deliverable: localizedWorkflowText{english: "The improved artifact with a verified change and handoff record.", chinese: "改进后的成果及已验证的变更与交接记录。"}, acceptanceCriteria: []localizedWorkflowText{{english: "The delivered artifact is the exact revision that passed verification.", chinese: "交付成果与通过验证的修订版本完全一致。"}, {english: "Resolved and deferred findings are traceable.", chinese: "已解决与延后问题均可追溯。"}}, required: true, terminal: true},
		},
		dependencies: []protocol.WorkGraphWorkflowDependency{
			{LogicalKey: "quality-audit", DependsOnLogicalKey: "baseline", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "experience-audit", DependsOnLogicalKey: "baseline", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "prioritize", DependsOnLogicalKey: "quality-audit", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "prioritize", DependsOnLogicalKey: "experience-audit", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "revise", DependsOnLogicalKey: "prioritize", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "verify", DependsOnLogicalKey: "revise", Kind: protocol.WorkDependencyHard},
			{LogicalKey: "deliver", DependsOnLogicalKey: "verify", Kind: protocol.WorkDependencyHard},
		},
	},
}

func builtinWorkflows(locale string) []protocol.WorkGraphWorkflow {
	result := make([]protocol.WorkGraphWorkflow, 0, len(builtinWorkflowDefinitions))
	for _, definition := range builtinWorkflowDefinitions {
		result = append(result, definition.workflow(locale))
	}
	return result
}

func builtinWorkflowByID(id string, locale string) *protocol.WorkGraphWorkflow {
	for _, definition := range builtinWorkflowDefinitions {
		if builtinWorkflowIDPrefix+definition.slashName == strings.TrimSpace(id) {
			workflow := definition.workflow(locale)
			return &workflow
		}
	}
	return nil
}

func builtinWorkflowBySlashName(slashName string, locale string) *protocol.WorkGraphWorkflow {
	for _, definition := range builtinWorkflowDefinitions {
		if definition.slashName == normalizeSlashName(slashName) {
			workflow := definition.workflow(locale)
			return &workflow
		}
	}
	return nil
}

func isBuiltinWorkflowID(id string) bool {
	return builtinWorkflowByID(id, "en") != nil
}

func isBuiltinWorkflowSlashName(slashName string) bool {
	return builtinWorkflowBySlashName(slashName, "en") != nil
}

func (definition builtinWorkflowDefinition) workflow(locale string) protocol.WorkGraphWorkflow {
	nodes := make([]protocol.WorkGraphWorkflowNode, 0, len(definition.nodes))
	for position, node := range definition.nodes {
		criteria := make([]string, 0, len(node.acceptanceCriteria))
		for _, criterion := range node.acceptanceCriteria {
			criteria = append(criteria, criterion.localized(locale))
		}
		nodes = append(nodes, protocol.WorkGraphWorkflowNode{
			LogicalKey: node.logicalKey, Role: node.role, Kind: node.kind,
			Subject: node.subject.localized(locale), Objective: node.objective.localized(locale),
			Deliverable: node.deliverable.localized(locale), AcceptanceCriteria: criteria,
			Required: node.required, Terminal: node.terminal,
			ParentLogicalKey: node.parentLogicalKey, Position: position,
		})
	}
	completionCriteria := make([]string, 0, len(definition.completionCriteria))
	for _, criterion := range definition.completionCriteria {
		completionCriteria = append(completionCriteria, criterion.localized(locale))
	}
	return protocol.WorkGraphWorkflow{
		ID: builtinWorkflowIDPrefix + definition.slashName, BuiltIn: true,
		SlashName: definition.slashName, Title: definition.title.localized(locale),
		Description: definition.description.localized(locale), Objective: definition.objective.localized(locale),
		CompletionCriteria: completionCriteria, Nodes: nodes,
		Dependencies: slices.Clone(definition.dependencies), Version: 1,
		CreatedAt: builtinWorkflowPublishedAt, UpdatedAt: builtinWorkflowPublishedAt,
	}
}
