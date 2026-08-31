// Package agent 提供 Agent 业务能力：CRUD、独立业务标签、运行时提示词构建、workspace/skills 就绪。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / crud.go / creation_request.go / deletion_coordination.go / update_reconciliation.go / ready.go /
//     skills.go / workspace.go：Service、分阶段 Agent 更新、跨域删除协调、Skill
//     全局启用与 workspace 停用列的独立原子更新、runtime_version CAS 与计数；删除先
//     提交数据库身份撤销，再由 app 协调 Channel 与全部 DM/Room runtime 墓碑撤销，
//     后置失败显式返回 reconcile 状态；更新在主写入提交后发生的投影失败也保留
//     committed 证据；删除前的控制面拒绝使用稳定 sentinel，
//     Handler 不解析错误文案判断提交事实。带 creation_request_id 的创建先持久
//     owner-scoped reservation，再以 workspace stage/claim fence 和同事务 Agent 回执提交实现 exact 恢复；
//     无 request ID 调用保留旧语义。
//   - contacts.go：同 owner 普通 Agent 的双向联系人、别名与直聊 Room 绑定。
//   - prompt_build.go：BuildRuntimePrompt 运行时附加提示词、默认模板与主智能体委派边界；
//     workspace AGENTS.md 只由 SDK 加载，Nexus 不做第二次 prompt 拼接。
//   - repository.go / factory_record.go：持久化、默认平台 Skill 引用与记录构造。
//   - emotion_state.go / runtime_settings.go：带 Agent scope 锁与 version CAS 的 runtime 情绪态及 nxs 非敏感主模型 settings 投影；owner 后台模型只在 bridge 启动环境中投影。
//   - policy_name.go / scope_owner.go / workspace_path.go：命名策略、归属与 workspace 路径。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package agent
