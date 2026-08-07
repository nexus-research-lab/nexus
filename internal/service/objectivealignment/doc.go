// Package objectivealignment 定义 Goal completion 与 Execution loop guard 共用的
// 可审计目标对齐契约。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - audit.go：权威 target、逐 criterion 报告规范化、三态聚合与 fingerprint。
//   - prompt.go：模型执行同一审计协议所需的稳定说明。
//
// 本包不读取 Goal、Execution、Plan 或 NodeRun，不持久化状态，也不选择控制流；
// 各生命周期只保存并消费这里验证后的 ObjectiveAlignmentReport。
package objectivealignment
