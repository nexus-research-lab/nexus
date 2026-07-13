// Package workspace 提供 Agent workspace 的文件读写、上传与实时同步。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / file.go / memory.go / mutation.go / upload.go / path.go：Service、文件、记忆投影、分阶段条目变更、上传、路径。
//   - agent.go / model.go / reveal.go：Agent workspace、模型、本机定位。
//   - initializer.go / initializer_*.go：workspace 初始化阶段与主 Agent 文件策略（模板 / skill / nexusctl / 模板集）。
//   - live.go / live_*.go：实时文件树模型与同步阶段（行级 diff / watcher / write）。
//   - upload_dedupe.go：上传去重。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
