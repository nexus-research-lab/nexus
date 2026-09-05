// INPUT: 预览文件的 exact Agent 与不同的全局选择。
// OUTPUT: 证明 Markdown 图片从文件归属生成 URL，切换文件来源后立即更新。
// POS: Workspace 文本正文到共享 Markdown 能力注入的实际消费行为测试。

import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { useAgentStore } from "@/store/agent";

import { TextFileContent } from "./text-file-content";

afterEach(() => {
  cleanup();
  useAgentStore.setState({ current_agent_id: null });
});

it("previews images for the file Agent even when another Agent is selected", () => {
  useAgentStore.setState({ current_agent_id: "selected-agent" });
  const props = { content: "![Chart](images/chart.png)", fileName: "report.md", fileType: "markdown" as const, isLoading: false, isStreaming: false };
  const { rerender } = render(<TextFileContent {...props} agentId="file-agent" />);
  expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/file-agent/workspace/download?");

  act(() => useAgentStore.setState({ current_agent_id: "another-selection" }));
  expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/file-agent/workspace/download?");
  rerender(<TextFileContent {...props} agentId="new-file-agent" />);
  expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/new-file-agent/workspace/download?");
});
