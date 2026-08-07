// Package storage 负责数据库连接打开与 migration 目录/方言解析。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - database.go：OpenDB 打开带 immediate transaction、busy timeout 与外键保护的运行连接；OpenMigrationDB 提供 migration 专用连接。
//   - dialect.go：MigrationDirName / GooseDialect 解析驱动的迁移目录与 goose 方言。
//
// 暴露接口：OpenDB、OpenMigrationDB、MigrationDirName、GooseDialect。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package storage
