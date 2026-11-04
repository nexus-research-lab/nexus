// INPUT: 数据库事务维护的目录结构版本。
// OUTPUT: 已提交的跨进程目录版本与 Agent 展示版本，不扫描房间或消息历史。
// POS: 聊天目录失效信号的存储读取边界。
package roomrepo

import "context"

// DirectoryVersion 是目录 mutation 提交后可见的单调版本；Agent 是其中 Agent/Profile 展示变化的子版本。
type DirectoryVersion struct {
	Directory int64
	Agent     int64
}

// DirectoryVersion 只读取触发器随目录 mutation 提交的版本。
func (r *SQLRepository) DirectoryVersion(ctx context.Context) (DirectoryVersion, error) {
	var version DirectoryVersion
	err := r.db.QueryRowContext(ctx, `SELECT revision, agent_revision FROM directory_revision WHERE id = 1`).
		Scan(&version.Directory, &version.Agent)
	return version, err
}
