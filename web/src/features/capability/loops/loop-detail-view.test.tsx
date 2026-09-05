// INPUT: 一个完整 Loop 快照、共享本地化与复制适配器。
// OUTPUT: 证明详情页使用共享 Button、Badge、Panel 与语义 Typography，并保留复制行为。
// POS: Loop 详情 DOM 合同；请求竞态与目录快照由资源可靠性测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { LoopCatalogItem } from "@/types/capability/loop";

import { LoopDetailView } from "./loop-detail-view";

const mocks = vi.hoisted(() => ({
  getLoopApi: vi.fn(),
  t: (key: string) => key,
  writeTextToClipboard: vi.fn(),
}));

vi.mock("@/lib/api/capability/loop-api", () => ({
  getLoopApi: mocks.getLoopApi,
}));

vi.mock("@/shared/lib/browser/clipboard", () => ({
  writeTextToClipboard: mocks.writeTextToClipboard,
}));

vi.mock("@/shared/i18n/i18n-context", () => ({
  useI18n: () => ({
    locale: "zh",
    t: mocks.t,
  }),
}));

const LOOP: LoopCatalogItem = {
  id: "quality-loop",
  slug: "quality-loop",
  title: "Quality Loop",
  description: "Iterate until the delivery is verified.",
  category: "Quality",
  trigger_type: "manual",
  trigger_config: {},
  steps: [{
    name: "Inspect",
    prompt: "Review the current result.",
    shell_check: "npm test",
  }],
  exit_condition: {
    type: "command",
    command: "npm test",
    description: "Stop after validation passes.",
    max_iterations: 3,
  },
  kickoff_prompt: "Inspect, revise, and validate.",
  install_bundle: {},
  compatible_agents: [],
  best_for_agents: [],
  author: "Nexus",
  author_slug: "nexus",
  author_official: true,
  source: "builtin",
  tags: ["delivery"],
  guardrails: ["Do not skip validation."],
  examples: [],
  copies: 0,
  installs: 0,
  views: 0,
  featured: false,
  is_published: true,
  created_at: "2026-09-03T00:00:00Z",
};

describe("LoopDetailView", () => {
  beforeEach(() => {
    mocks.getLoopApi.mockReset();
    mocks.getLoopApi.mockResolvedValue(LOOP);
    mocks.writeTextToClipboard.mockReset();
    mocks.writeTextToClipboard.mockResolvedValue(true);
  });

  it("renders Loop chrome through shared semantic owners", async () => {
    const { container } = render(
      <LoopDetailView onBack={vi.fn()} slug={LOOP.slug} />,
    );

    expect((await screen.findByRole("heading", { name: LOOP.title })).className).toContain("ui-type-object-title");
    expect(screen.getByRole("heading", { name: "capability.loops_steps" }).className).toContain("ui-type-section-title");
    expect(screen.getByRole("heading", { name: "Inspect" }).className).toContain("ui-type-control");
    expect(screen.getByText("Review the current result.").className).toContain("ui-type-supporting");
    expect(screen.getAllByText("npm test")[0].className).toContain("ui-type-code");
    expect(screen.getByText("Quality").className).toContain("text-2xs");
    expect(container.querySelectorAll(".surface-radius-sm").length).toBeGreaterThan(1);
    expect(screen.getByRole("button", { name: "capability.loops" }).className).toContain("ui-type-metadata");
    expect(container.querySelector("[data-slot='capability-detail-header']")).toBeTruthy();
    expect(container.querySelector("[data-slot='capability-detail-identity']")).toBeTruthy();
    expect(container.querySelector(
      "[data-slot='capability-detail-identity-leading'] .radius-control-lg",
    )).toBeTruthy();
    expect(screen.getAllByText(LOOP.title).some((node) => (
      node.className.includes("ui-type-metadata")
    ))).toBe(true);
  });

  it("copies the kickoff prompt through the shared clipboard adapter", async () => {
    const user = userEvent.setup();
    render(<LoopDetailView onBack={vi.fn()} slug={LOOP.slug} />);

    await user.click(await screen.findByRole("button", {
      name: "capability.loops_copy_prompt",
    }));

    expect(mocks.writeTextToClipboard).toHaveBeenCalledWith(LOOP.kickoff_prompt);
  });
});
