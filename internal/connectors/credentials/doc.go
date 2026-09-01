// Package credentials 处理连接器凭据的加密密钥编解码、身份与有限历史 keyring。
//
// L2 | 父级: internal/connectors（L1 见 AGENTS.md）
//
// 成员清单：
//   - codec.go：DecodeKey 解析 32 字节 base64 加密密钥。
//   - keyring.go：固定 active writer、稳定 key_id、自带身份的 envelope 与缺少身份的旧密文兼容读取。
//   - host_keys*.go：显式/文件/平台 Keychain 宿主来源解析；只在启动期选择 active/legacy keys。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package credentials
