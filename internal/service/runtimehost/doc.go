// Package runtimehost 持有 DM 与 Room realtime 共用的宿主运行依赖与阶段。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - host.go：Host 依赖集合、注入方法，以及额度、执行上下文、Slash 展开、生图默认能力与日志等共用阶段。
//   - goal_round.go：GoalRoundState，DM round 与 Room slot 共用的每轮 Goal 状态（一把 Mu）及子任务、完成收据、assistant 记录与 token 用量写入去重。
//   - session.go：owner 后台任务、精确 owner 上下文、fork runtime 回收、工具面换代与队列条目恢复。
//   - round_mapper.go：把共用 message.EventMapper 适配为 exec.RoundMapper。
//   - goal.go：Goal 续跑标记/结算、完成收据读取、额度限制标记、terminal 用量持久重试、可信投递策略与 SDK 诊断日志装配。
//
// DM 是 Room 的一种：两者的 Service 都嵌入 Host，共用阶段只实现一次；Room 只在
// realtime 包内注入多成员 slot 与公私域策略。本包不得依赖 dm 或 room/realtime。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtimehost
