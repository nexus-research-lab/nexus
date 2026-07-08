// Package providers 定义各连接器 OAuth Provider 与注册表。
//
// L2 | 父级: internal/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - provider.go / registry.go：Provider 抽象与注册表。
//   - feishu_docx.go / github.go / google.go / twitter.go / linkedin.go /
//     instagram.go / shopify.go：各平台 OAuth Provider。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package providers
