/** 识别当前 nexus.show_widget 与历史 transcript 中的旧包装名。 */
const GENERATIVE_UI_TOOL_NAMES = new Set([
  "show_widget",
  "mcp__nexus__show_widget",
  "nexus__show_widget",
  "nexus.show_widget",
  "nexus/show_widget",
  "mcp__nexus_visualize__show_widget",
  "nexus_visualize__show_widget",
  "nexus_visualize.show_widget",
  "nexus_visualize/show_widget",
]);

export function isGenerativeUIWidgetToolName(toolName: string): boolean {
  return GENERATIVE_UI_TOOL_NAMES.has(toolName.trim());
}
