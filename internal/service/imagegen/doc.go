// Package imagegen 提供图片生成能力，按 provider 解析配置并适配多家后端。
//
// L2 | 父级: internal/service（L1 见 AGENTS.md）
//
// 成员清单：
//   - service.go / service_config.go / service_input.go / service_output.go：Service 及输入/输出/配置。
//   - service_provider_openai.go / provider_dashscope.go / provider_modelscope.go / service_provider_options.go：各 provider 适配与选项。
//   - service_http.go / service_image_extract.go：HTTP 调用与图片提取。
//   - model_imagegen.go：图片生成模型。
//
// [PROTOCOL]: 变更时更新此头部，然后检查父级入口 AGENTS.md（L1）
package imagegen
