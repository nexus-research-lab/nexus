// Package credentials 处理连接器凭据的加密密钥编解码。
//
// L2 | 父级: internal/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - codec.go：DecodeKey 解析 32 字节 base64 加密密钥。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package credentials
