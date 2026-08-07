// Package channels 编排 IM 通道的入站、路由、账号配置、登录与配对。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - ingress*.go：入站接收、消息归一化、投递目标解析、权限与会话映射。
//   - router.go / router_*.go：generation 防护的通道路由、候选先启动后替换、
//     投递记录与平台配置注册表。
//   - channel_*.go / existence.go：通道账号、配置存储与写后精确存在性核验。
//     catalog 标记为 secret 的字段禁止进入普通 config JSON；读取旧数据时也按
//     catalog 过滤。候选 runtime 失败时恢复旧内容但发布新的单调 control version，
//     使失败前后的旧 plan 都无法重新命中。
//   - login*.go / pairing*.go：微信登录、官方应用扫码注册与字段 patch 配对；
//     登录完成绑定启动时 control version 和可选账号，凭据写入使用 CAS，候选
//     runtime 启动失败恢复授权前配置；对话授权凭据提交持有可撤销 lease，
//     精确取消可等待 poller 离开写路径；所有 pairing writer 共用 owner 锁。
//   - control.go / control_*.go / mutation_lock.go：通道控制、凭据与值归一化及
//     owner + channel 串行写边界。
//   - session_delivery.go / room_delivery.go：会话与房间主动投递。
//   - model_channel.go / model_control.go：通道与控制模型。
//
// 具体平台适配见子包 adapters/；通道无关契约见 contract/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package channels
