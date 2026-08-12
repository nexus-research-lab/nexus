// Package channels 编排 IM 通道的入站、路由、账号配置、登录与配对。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - ingress*.go：入站接收、消息归一化、投递目标解析、权限与会话映射；active-paired 私聊签发同 Agent 交互能力，把普通 runtime permission 投递为不暴露内部 ID 的 session-scoped `/y`、`/a`、`/d` 确认，可信控制命令在 Agent runtime 前跨 runtime/Automation 统一消歧、消费并回投当前 IM。
//   - router.go / router_*.go：generation 防护的通道路由、候选先启动后替换、
//     投递记录与平台配置注册表。
//   - channel_*.go / existence.go：通道账号、配置存储与写后精确存在性核验。
//     catalog 标记为 secret 的字段禁止进入普通 config JSON；读取旧数据时也按
//     catalog 过滤。候选 runtime 失败时恢复旧内容但发布新的单调 control version，
//     使失败前后的旧 plan 都无法重新命中。
//   - login*.go / pairing*.go / pairing_delivery.go：微信登录、官方应用扫码注册、字段 patch 配对与所有外部 IM ingress/Automation 共用的 active pairing 实时授权及稳定不可用错误；
//     登录完成绑定启动时 control version 和可选账号，凭据写入使用 CAS，候选
//     runtime 启动失败恢复授权前配置；对话授权凭据提交持有可撤销 lease，
//     精确取消可等待 poller 离开写路径；所有 pairing writer 共用 owner 锁。
//   - control.go / control_*.go / mutation_lock.go：通道控制、凭据与值归一化及
//     owner + channel 串行写边界。
//   - external_session_identity.go：把 pairing/account 真相投影为不泄露账号原值的 Session 短标识、当前/历史状态与初始删除资格。
//   - session_delivery.go / room_delivery.go / automation_delivery.go：会话与房间主动投递；Automation 结果用 run_id 幂等投影到逻辑会话，再发送外部 IM 并关联平台回执。
//   - model_channel.go / model_control.go：通道与控制模型。
//
// 具体平台适配见子包 adapters/；通道无关契约见 contract/。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package channels
