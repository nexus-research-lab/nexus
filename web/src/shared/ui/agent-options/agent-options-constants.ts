/**
 * AgentOptions 共享常量
 *
 * 提取到独立文件以避免 react-refresh/only-export-components 警告
 */

/** 预定义的模型列表 */
export const AVAILABLE_MODELS = [
  { value: "glm-5", label: "GLM 5" },
  { value: "deepseek-chat", label: "DeepSeek Chat | 深度求索" },
  { value: "claude-3-5-sonnet", label: "Claude 3.5 Sonnet" },
  { value: "claude-3-opus", label: "Claude 3 Opus" },
  { value: "claude-3-haiku", label: "Claude 3 Haiku" },
];
