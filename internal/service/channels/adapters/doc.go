// Package adapters 实现各 IM 平台的通道适配（回调、投递、入站、鉴权、模型）。
//
// L2 | 父级: internal/service/channels（L1 见 AGENTS.md）
//
// 成员清单（按平台）：
//   - dingtalk*.go：钉钉（回调 / 投递 / stream）。
//   - feishu_*.go：飞书（API / 回调安全 / 入站 / token / 模型）。
//   - telegram*.go：Telegram（投递 / 入站 / polling / 模型）。
//   - wecom_bot_*.go：企业微信机器人（callback stream 即时回复 / aibot_send_msg 延迟主动投递 /
//     有界 socket 写入 / 连续缺失 pong 的半开连接重连 / 入站）。
//   - personal_weixin_*.go：个人微信（client / multi / 模型 / 启停通知 / 持久轮询游标 /
//     -14 登录失效收口 / 明确失效后的 context_token 单次降级）。
//   - discord.go / support.go：Discord 与共享辅助。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package adapters
