// Package confinedfs 提供以 os.Root/目录文件描述符为边界的宿主文件访问。
//
// 所有来自 workspace、runtime artifact、transcript 和用户 Skill 的相对路径
// 都应先绑定到本包的 Root，再进行读写、遍历、重命名或删除；调用方不应在
// owner 校验后把原始绝对路径重新交给 os.*。
package confinedfs
