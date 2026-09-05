// INPUT: Skill 包说明中的外链、相对图片与不相关的当前 Agent。
// OUTPUT: 证明说明保留自身资源引用，不借 Workspace 当前选择改写图片。
// POS: Skill Markdown 实际消费侧的资源语义测试。

import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";

import { useAgentStore } from "@/store/agent";

import { SkillMarkdown } from "./skill-markdown";

afterEach(() => {
  cleanup();
  useAgentStore.setState({ current_agent_id: null });
});

it("preserves package resource paths and safe external links independently of Agent selection", () => {
  useAgentStore.setState({ current_agent_id: "selected-agent" });
  render(<SkillMarkdown markdown="[Reference](https://example.com/skill) ![Package image](images/example.png) ![Remote image](https://example.com/example.png)" />);
  expect(screen.getByRole("link", { name: "Reference" }).getAttribute("href")).toBe("https://example.com/skill");
  expect(screen.getByRole("img", { name: "Package image" }).getAttribute("src")).toBe("images/example.png");
  expect(screen.getByRole("img", { name: "Remote image" }).getAttribute("src")).toBe("https://example.com/example.png");

  act(() => useAgentStore.setState({ current_agent_id: "other-agent" }));
  expect(screen.getByRole("img", { name: "Package image" }).getAttribute("src")).toBe("images/example.png");
  expect(screen.queryByRole("button")).toBeNull();
});
