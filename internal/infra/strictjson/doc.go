// Package strictjson 提供跨包共用的严格 JSON 结构校验原语（重复键、嵌套深度、尾随数据）。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - keys.go：ValidateKeys，在解码前拒绝重复/非法键、超深嵌套与尾随数据。
//
// 本包只依赖标准库。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package strictjson
