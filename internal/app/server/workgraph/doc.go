// Package workgraph 装配 WorkGraph 隐藏编辑 Session 与隔离保存 round。
//
// L2 | 父级: internal/app/server（L1 见 AGENTS.md）
//
// 成员清单：
//   - adapter.go：创建/删除隐藏编辑 Session，并在隔离内部 DM 中运行保存 round。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package workgraph
