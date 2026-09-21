// Package relay 提供 Nexus Server 到 Nexus Relay v1 的 typed HTTP/WSS client。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - client.go：Bearer 认证、在线 Room 创建/列表、幂等消息提交、快照/增量查询、共用 WSS transport 的真人水位/节点任务提示订阅与 upgrade 错误解码。
//   - membership_client.go：Room 管理快照、真人邀请、角色、移除与群主移交 transport。
//   - delivery_client.go：Node 只读任务提示、幂等领取、续期、失败与完整输出；不执行工具。
//   - files.go：固定 Room 文件路径的上传、下载和列表传输，以及投递租约约束下校验大小与摘要的附件读取；本包不写本机目录。
//   - wire 合同统一位于 internal/relay，本包只处理远端传输。
//
// 暴露接口：Client、NewClient；方法消费 internal/relay 的独立合同。
// WS 复用原生 Ping/Pong 检测半开连接，失败交由调用方恢复；不在 transport 重放业务命令。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package relay
