// Package userroot 迁移 Nexus users 根，并协调重启时的数据库路径切换。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go：设置保存时只校验并登记目标，不触碰运行中的用户文件。
//   - startup.go：启动无并发窗口迁移完整 users 树、原子切换 Agent 路径，失败时回退旧根。
//   - files.go：以目录文件描述符固定新旧根，完成跨卷安全复制、transcript 项目重映射与路径校验。
//   - metadata.go：重映射 Room 宿主状态中的结构化 workspace 绝对路径。
//
// 暴露接口：Manager、NewManager、Manager.Schedule、ReconcileOnStartup。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package userroot
