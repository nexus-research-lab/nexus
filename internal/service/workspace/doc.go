// Package workspace 提供 Agent workspace 的文件读写、上传与实时同步。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_file.go / service_mutation.go / service_upload.go / service_path.go：Service、文件、增删改、上传、路径。
//   - service_agent.go / service_model.go / service_reveal.go：Agent workspace、模型、本机定位。
//   - initializer_*.go：workspace 初始化（模板 / skill / nexusctl / 模板集）。
//   - manager_live*.go / model_live.go：实时文件树同步（diff / watcher / write）。
//   - upload_dedupe.go：上传去重。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package workspace
