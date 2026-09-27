// Package runtimecheck 装配安装包内 sidecar 与 nxs 的无模型兼容性检查。
//
// L2 | 父级: internal/app（L2 见 ../doc.go）
//
// 成员清单：
//   - check.go：使用产品 clientopts、固定 Bridge 与临时状态执行真实 initialize/close。
//
// 不加载 .env、宿主数据库或用户配置，不发起模型请求；不代替完整沙箱或升级验收。
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 ../doc.go（L2）
package runtimecheck
