// Package channels 编排 IM 通道的入站、路由、账号配置、登录与配对。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - ingress*.go：入站接收、归一化、投递目标解析、权限、会话映射。
//   - router*.go / router_config.go：通道路由生命周期与平台配置注册表。
//   - channel_*.go：通道账号与配置存储。
//   - login*.go / pairing*.go：表驱动登录流程与设备配对（含 weixin/feishu）。
//   - control*.go：通道控制。
//   - session_delivery.go / room_delivery.go：会话与房间主动投递。
//   - model_channel.go / model_control.go / message_migration.go：模型与消息迁移。
//
// 具体平台适配见子包 adapters/；通道无关契约见 contract/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package channels
