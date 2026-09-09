// Package projectpermission 将 owner-scoped 项目成员操作转换为受信 launcher 命令。
//
// 业务服务不直接修改 /etc/passwd、系统组或 POSIX ACL；Linux enforce 之外
// 明确返回 unavailable，避免形成一套仅靠数据库标记的伪隔离语义。成员关系
// 变化后统一回收该 owner 的热 runtime，防止旧进程继续持有已撤销的项目 GID。
// Available 与 launcher 调用共用 Linux/enforce/绝对可执行路径判定，避免 UI 与执行边界漂移。
package projectpermission
