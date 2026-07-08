// Package sessionresume 定义 Nexus 产品层的 resume 状态策略。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 职责边界：
//   - Nexus 产品层：维护 DM/Room 入口、持久化 session 元数据、判断 resume id 是否有
//     对应 transcript 支撑。
//   - bridge：封装 runtime 进程、配置、连接、消息流和 stderr 诊断。
//   - SDK：定义 bridge 暴露的协议类型和控制面接口。
//
// 这个包不能依赖 bridge client 或 SDK wire 类型，避免把产品入口状态下沉到中间层。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package sessionresume
