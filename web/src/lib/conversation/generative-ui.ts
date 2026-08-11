/** 识别 Nexus 内建 show_widget 在不同 runtime 中的稳定包装名。 */
const GENERATIVE_UI_TOOL_NAMES = new Set([
  "show_widget",
  "mcp__nexus_visualize__show_widget",
  "nexus_visualize__show_widget",
  "nexus_visualize.show_widget",
  "nexus_visualize/show_widget",
]);

export function isGenerativeUIWidgetToolName(toolName: string): boolean {
  return GENERATIVE_UI_TOOL_NAMES.has(toolName.trim());
}
