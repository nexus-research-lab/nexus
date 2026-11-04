// INPUT: 数据库事务维护的目录结构版本。
// OUTPUT: 已提交的跨进程目录版本，不扫描房间或消息历史。
// POS: 聊天目录失效信号的存储读取边界。
package roomrepo

import "context"

// DirectoryRevision 只读取触发器与目录 mutation 同事务提交的版本。
func (r *SQLRepository) DirectoryRevision(ctx context.Context) (int64, error) {
	var revision int64
	err := r.db.QueryRowContext(ctx, `SELECT revision FROM directory_revision WHERE id = 1`).Scan(&revision)
	return revision, err
}
