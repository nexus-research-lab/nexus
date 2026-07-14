// Package provider 管理 Provider 配置并解析 Agent 最终运行时使用的 Provider。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / listing.go / store_view.go / visibility.go：Service、列举、存储视图、可见性。
//   - mutation.go / record.go / normalize.go / helpers.go：增删改、记录、规整、辅助。
//   - model_*.go：模型卡发现、内置已知窗口、拉取、校验、编解码、选择，以及分阶段变更与清洗。
//   - runtime_config.go / image_config.go：Agent 运行时解析与分阶段图片 Provider/模型选择。
//   - catalog_provider.go / model_provider.go：目录与模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
