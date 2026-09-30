// Package runtimebootstrap 校验 macOS App 随包监督 helper。
//
// L2 | 父级: internal/infra（L1 见 AGENTS.md）
// bundle.go 从 sidecar 所在 Contents 派生固定资源位置，经 confinedfs 拒绝链接，
// 核对签名后摘要、Bridge 构建版本、native cgo 与当前架构。
// current.go 的 LoadCurrent 从当前 sidecar 自身构建信息取得固定依赖版本。
// 本包不下载或选择外部 helper；清单真实性依赖调用方的可信 App/签名边界。
package runtimebootstrap
