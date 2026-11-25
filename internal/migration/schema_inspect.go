// INPUT: 已打开的 migration 数据库、驱动与待探测的列/索引清单。
// OUTPUT: 已存在数量与排序后的缺失项，供兼容修复判断 schema 是否完整。
// POS: 启动前兼容修复共用的 schema 探测；只读，不修改账本。
package migration

import (
	"context"
	"database/sql"
	"sort"
)

type migrationSchemaColumn struct {
	table  string
	column string
}

func inspectMigrationSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
	columns []migrationSchemaColumn,
	indexes []string,
) (int, []string, error) {
	present := 0
	missing := make([]string, 0)
	for _, field := range columns {
		exists, err := migrationColumnExists(ctx, driver, db, field.table, field.column)
		if err != nil {
			return 0, nil, err
		}
		if exists {
			present++
		} else {
			missing = append(missing, field.table+"."+field.column)
		}
	}
	for _, index := range indexes {
		exists, err := migrationIndexExists(ctx, driver, db, index)
		if err != nil {
			return 0, nil, err
		}
		if exists {
			present++
		} else {
			missing = append(missing, index)
		}
	}
	sort.Strings(missing)
	return present, missing, nil
}
