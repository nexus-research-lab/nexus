// Package welcomegen 为新物化的 DM 与 Room 主会话生成一次宿主欢迎语。
//
// # L2 — 成员清单
//
// - service.go：异步任务生命周期、去重与实时失效广播。
// - generation.go：发言 Agent 选择、后台模型优先级、四类身份提示词、身份校验与静态回退。
// - persistence.go：DM/Room 历史幂等写入。
// - contract.go：Agent、Provider、Preferences 与广播消费侧窄接口。
//
// # L2 — 暴露接口
//
// - NewService：装配欢迎语生成服务。
// - Service.Schedule：接收新建 conversation 聚合并异步生成欢迎语。
// - Service.SetLogger / SetRoomResyncBroadcaster / Close：应用生命周期集成。
package welcomegen
