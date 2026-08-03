// Package appfs 解析 Nexus 全局配置目录与根路径。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - config_dir.go：StateRoot、固定 AppDir、可切换 UsersRoot、用户路径与平台运行时共享目录。
//   - root.go：Root 配置根。
//   - runtime_permissions.go：runtime 共享临时根，以及 Linux enforce 下宿主与 owner 私有组的协作 mode。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package appfs
