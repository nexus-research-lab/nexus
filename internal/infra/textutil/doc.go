// Package textutil 提供跨包共用的字符串取值原语，替代各包私有的同构副本。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - text.go：FirstNonEmpty、PointerValue、AnyString。
//
// 本包只依赖标准库，是架构门禁允许 runtime 根包导入的叶子包；不得引入任何 internal 依赖。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package textutil
