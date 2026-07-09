// Package storage 负责数据库连接打开与 migration 目录/方言解析。
//
// L2 | 父级: internal（L1 见 AGENTS.md）
//
// 成员清单：
//   - database.go：OpenDB 打开当前配置对应的数据库连接。
//   - dialect.go：MigrationDirName / GooseDialect 解析驱动的迁移目录与 goose 方言。
//
// 暴露接口：OpenDB、MigrationDirName、GooseDialect。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package storage
