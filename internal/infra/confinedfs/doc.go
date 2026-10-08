// Package confinedfs 提供以 os.Root/目录文件描述符为边界的宿主文件访问。
//
// 所有来自 workspace、runtime artifact、transcript 和用户 Skill 的相对路径
// 都应先绑定到本包的 Root，再进行读写、遍历、重命名或删除；调用方不应在
// owner 校验后把原始绝对路径重新交给 os.*。并发敏感的文件可在临时文件
// 已完整落盘后复核旧正文，变更时不执行最终替换。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
// root.go 提供目录句柄边界；quarantine_darwin.go 在固定 parent 根间以
// RENAME_EXCL 隔离原目录并核对移动后身份，SyncDirectory 支持宿主恢复提交。
// 移动后报错可能已产生文件动作，调用者必须从同一回收目标对账，不能盲重试新目标。
package confinedfs
