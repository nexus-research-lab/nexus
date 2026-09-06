// INPUT: 真实 Loop Dialog、读取结果、搜索/分类与单次启动回调。
// OUTPUT: 初始搜索焦点、筛选清理、准确启动和资源恢复不被样式修改影响。
// POS: 开放选择器集成测试，网络读取和启动命令隔离。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import type { LoopCatalogItem } from "@/types/capability/loop";
import { LoopPickerDialog } from "./loop-picker-dialog";
import { LoopPickerContent } from "./loop-picker-content";

const list = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/capability/loop-api", () => ({ listLoopsApi: list }));
const LOOP: LoopCatalogItem = { id: "verify", slug: "verify", title: "Verify release", category: "Quality", description: "Review the full result.",
  trigger_type: "manual", tags: ["delivery"], compatible_agents: ["Reviewer"], trigger_config: {}, steps: [],
  exit_condition: { type: "manual", description: "Stop after review." }, kickoff_prompt: "Verify", install_bundle: {}, best_for_agents: [],
  author: "Nexus", author_slug: "nexus", author_official: true, source: "builtin", guardrails: [], examples: [],
  copies: 0, installs: 0, views: 0, featured: false, is_published: true, created_at: "" };
const OTHER = { ...LOOP, slug: "research", title: "Research topic", category: "Research", tags: [] };
beforeEach(() => {
  localStorage.setItem(LOCALE_STORAGE_KEY, "en");
  list.mockReset().mockResolvedValue([LOOP, OTHER]);
});

describe("Loop picker", () => {
  it("focuses search, preserves matching fields, clears filters and starts exactly the chosen Loop", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onSelect = vi.fn(async () => undefined);
    render(<LoopPickerDialog isOpen onClose={onClose} onSelect={onSelect} />, { wrapper: I18nProvider });
    const search = screen.getByLabelText("Search loops, tags, or categories...");
    await waitFor(() => expect(document.activeElement).toBe(search));
    await screen.findByRole("button", { name: /Verify release/ });
    await user.type(search, "delivery");
    expect(screen.queryByRole("button", { name: /Research topic/ })).toBeNull();
    await user.clear(search);
    await user.type(search, "missing");
    expect(screen.getByText("No matching loops")).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Clear filters" }));
    const row = screen.getByRole("button", { name: /Research topic/ });
    row.focus();
    await user.keyboard("{Enter}");
    expect(onSelect).toHaveBeenCalledExactlyOnceWith(OTHER);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("keeps stale rows on an ordinary refresh error but hides them when access is revoked", () => {
    const props = { busySlug: null, hasCatalogItems: true, hasSnapshot: true, isLoading: false,
      loops: [LOOP], onClearFilters: vi.fn(), onRetry: vi.fn(), onSelect: vi.fn() };
    const { rerender } = render(<LoopPickerContent {...props} error={{ access: null, message: "Offline" }} />,
      { wrapper: I18nProvider });
    expect(screen.getByRole("button", { name: /Verify release/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Retry" })).toBeTruthy();
    rerender(<LoopPickerContent {...props} error={{ access: "forbidden", message: "Access changed" }} />);
    expect(screen.queryByRole("button", { name: /Verify release/ })).toBeNull();
    expect(props.onSelect).not.toHaveBeenCalled();
  });
});
