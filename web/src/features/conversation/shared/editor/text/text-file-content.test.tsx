// INPUT: 预览文件的 exact Agent、不同的全局选择与未知格式纯文本。
// OUTPUT: Markdown 图片跟随文件归属，未知文本保留空白且不被解释成 DOM。
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

it("keeps unknown plain text literal and preserves whitespace without creating markup", () => {
  const content = "  leading\tspaces\n<img src='untrusted' />\n中文";
  const { container } = render(<TextFileContent agentId="file-agent" content={content} fileName="notes.unknown"
    fileType="text" isLoading={false} isStreaming={false} />);
  expect(container.querySelector("pre")?.textContent).toBe(content);
  expect(screen.queryByRole("img")).toBeNull();
});
