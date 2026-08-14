// Package runtimeidentity 实现 Linux root-owned runtime launcher 的身份、
// POSIX ACL、共享项目授权、环境收口与 Landlock 系统调用边界。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
//
// 成员清单：
//   - model_linux.go：registry、identity、project、ticket 与执行 policy 数据模型。
//   - config_linux.go：root-owned 固定配置与受信任调用方校验。
//   - registry_linux.go：稳定 identity/project registry 与启动票据。
//   - accounts_linux.go：不可登录 OS 用户、私有组和 UID/GID 分配。
//   - acl_linux.go：用户树与共享项目的 access/default POSIX ACL、runtime ACL 修复，以及无符号链接 fd 操作。
//   - hardlinks_linux.go：root 身份下的跨用户、跨项目与隔离根外硬链接校验。
//   - cgroup_linux.go：可信会话信号、cgroup v2 per-user 进程继承与 cgroup.kill 回收。
//   - project_linux.go：项目组创建、read/write 成员授权与撤销。
//   - policy_linux.go：会话所需最小组与路径 policy 计算。
//   - environment_linux.go：runtime 环境 allowlist、宿主秘密与原始 Nexus CLI capability 剥离。
//   - landlock_linux.go：exec 前的最终文件系统系统调用约束与安全路径解析。
//   - run_linux.go：launcher 管理命令（含 ACL 修复和会话信号）与降权执行主链。
//   - run_other.go：非 Linux 明确拒绝实现。
//
// 暴露接口：Run。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package runtimeidentity
