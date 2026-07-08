// Package provider 管理 Provider 配置并解析 Agent 最终运行时使用的 Provider。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_listing.go / service_store_view.go / service_visibility.go：Service、列举、存储视图、可见性。
//   - service_mutation.go / service_record.go / service_normalize.go / service_helpers.go：增删改、记录、规整、辅助。
//   - service_model_*.go：模型卡发现、拉取、校验、编解码、选择、变更、清洗。
//   - service_runtime_config.go / service_image_config.go：Agent 运行时与图片生成的 Provider 解析。
//   - catalog_provider.go / model_provider.go：目录与模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package provider
