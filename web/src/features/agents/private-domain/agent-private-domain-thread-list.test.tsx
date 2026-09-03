// INPUT: 两条 Agent 私域线程、当前选中线程与选择命令。
// OUTPUT: 证明目录行复用 ListRow 的选择语义、密度和点击行为。
// POS: Agent 私域线程列表 DOM 行为测试；标题、摘要与时间格式由纯模型负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { AgentPrivateThread } from "@/types/agent/private-domain";

import { PrivateThreadList } from "./agent-private-domain-thread-list";

const THREADS: AgentPrivateThread[] = [
  {
    agent_id: "owner",
    last_content_preview: "第一条摘要",
    message_count: 1,
    participant_agent_ids: ["owner", "richmail"],
    participants: [
      { agent_id: "owner", name: "Nexus" },
      { agent_id: "richmail", name: "RichMail" },
    ],
    peer_agent_ids: ["richmail"],
    scope: "direct",
    thread_id: "thread-1",
  },
  {
    agent_id: "owner",
    last_content_preview: "第二条摘要",
    message_count: 1,
    participant_agent_ids: ["owner", "analyst"],
    participants: [
      { agent_id: "owner", name: "Nexus" },
      { agent_id: "analyst", name: "Analyst" },
    ],
    peer_agent_ids: ["analyst"],
    scope: "direct",
    thread_id: "thread-2",
  },
];

describe("PrivateThreadList", () => {
  it("uses shared selectable rows and forwards the exact thread", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <PrivateThreadList
        agentId="owner"
        compact
        isLoading={false}
        localization={{ locale: "zh", t: (key) => key }}
        onSelect={onSelect}
        selectedThreadId="thread-1"
        threads={THREADS}
      />,
    );

    const selected = screen.getByRole("button", { name: /RichMail/ });
    const other = screen.getByRole("button", { name: /Analyst/ });
    expect(selected.getAttribute("aria-pressed")).toBe("true");
    expect(other.getAttribute("aria-pressed")).toBe("false");
    expect(selected.className).toContain("min-h-10");
    expect(selected.className).toContain("radius-control-md");

    await user.click(other);
    expect(onSelect).toHaveBeenCalledWith("thread-2");
  });
});
