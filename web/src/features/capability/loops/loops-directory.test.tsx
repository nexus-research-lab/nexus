// INPUT: 本地 Loop 目录快照、真实路由和隔离的剪贴板回调。
// OUTPUT: 行内复制仅复制目标指令，行主动作仍打开对应详情。
// POS: 目录交互回归；不执行 Loop 或真实剪贴板写入。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router-dom";
import { expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import type { LoopCatalogItem } from "@/types/capability/loop";

import { LoopsDirectory } from "./loops-directory";

const mocks = vi.hoisted(() => ({ list: vi.fn(), copy: vi.fn() }));
vi.mock("@/lib/api/capability/loop-api", () => ({ listLoopsApi: mocks.list }));
vi.mock("@/shared/lib/browser/clipboard", () => ({ writeTextToClipboard: mocks.copy }));

const LOOP: LoopCatalogItem = {
  id: "verify-loop", slug: "verify-loop", title: "Verification loop", description: "Check this delivery.",
  category: "Quality", trigger_type: "manual", trigger_config: {}, steps: [],
  exit_condition: { type: "manual", description: "Stop after review." },
  kickoff_prompt: "  Verify this exact artifact.\nKeep its source intact.  ", install_bundle: {},
  compatible_agents: [], best_for_agents: [], author: "Nexus", author_slug: "nexus", author_official: true,
  source: "builtin", tags: [], guardrails: [], examples: [], copies: 0, installs: 0, views: 0,
  featured: false, is_published: true, created_at: "2026-09-06T00:00:00Z",
};

function CurrentRoute() {
  const location = useLocation();
  return <output data-testid="route">{location.pathname}</output>;
}

it("keeps pointer and keyboard copying separate from directory navigation", async () => {
  localStorage.setItem(LOCALE_STORAGE_KEY, "en");
  mocks.list.mockResolvedValue([LOOP]);
  mocks.copy.mockResolvedValue(true);
  const user = userEvent.setup();
  render(<MemoryRouter initialEntries={["/capability/loops"]}><I18nProvider>
    <LoopsDirectory /><CurrentRoute />
  </I18nProvider></MemoryRouter>);
  const action = await screen.findByRole("button", { name: "Copy start prompt" });
  await user.click(action);
  await user.keyboard("{Enter}");
  expect(mocks.copy).toHaveBeenCalledTimes(2);
  expect(mocks.copy).toHaveBeenLastCalledWith(LOOP.kickoff_prompt);
  expect(screen.getByTestId("route").textContent).toBe("/capability/loops");
  await user.click(screen.getByRole("heading", { name: LOOP.title }));
  expect(screen.getByTestId("route").textContent).toBe("/capability/loops/verify-loop");
});
