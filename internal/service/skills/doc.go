// Package skills 提供全局技能库、Agent 启停、workspace 本地投影与 marketplace 检索能力。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / catalog.go / registry*.go / references.go / selection_identity.go / file.go / workspace.go / confined_files.go：
//     Service、用户全局 catalog、Agent 私有 workspace 投影、来源/存储投影、
//     Agent 使用矩阵、target_scope/source_identity、非破坏性原子开关、平台/外部 Skill 引用、
//     owner-scoped confined registry 与 Agent runtime_version CAS；workspace 开关不删除文件。
//   - catalog_mutation.go / catalog_publish.go：owner catalog 持久单调 version/CAS、
//     typed reconcile、旧目录备份与 DB/FS 原子发布补偿。
//   - marketplace_*.go：外部 marketplace 检索、预览、来源配置、staging 导入与单项/批量更新；
//     Git / URL / skills.sh / 私有 JSON 注册表提供 expected version 对话入口；私有来源 CRUD/导入与 Web/API 共用 owner catalog CAS，
//     私有 Bearer 凭据保持加密，健康检查只更新非功能元数据，批量写按项推进 version 并检测部分完成，upload/local path 保持 human-only。
//   - frontmatter.go / model.go：frontmatter 解析、正文投影、技能模型、AgentSkillState，
//     以及不进入用户目录/手动绑定的内部角色配置 Skill 名单。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package skills
