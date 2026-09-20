// Package configuration 保存配置 revision 的宿主私有持久密钥。
//
// L2 | 父级: internal/storage（L1 见 AGENTS.md）
//
// 成员清单：
//   - revision_key.go：迁移预留单例的原子初始化、跨进程读取与损坏拒绝；不导出密钥到业务快照、配置或审计。
//
// 暴露接口：NewRevisionKeyStore、RevisionKeyStore.Key（仅供宿主 configuration 服务）。
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 doc.go（L2）
package configuration
