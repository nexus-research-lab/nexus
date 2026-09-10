// INPUT: Real catalog projections, local filters and directory callbacks.
// OUTPUT: Exclusive empty states, named controls, exact filter reset and read-only row actions.
// POS: Memory catalog DOM regression; file writes and deletion recovery stay with their controllers.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { MemorySnapshot } from "@/types/memory/memory";
import { AgentMemoryCatalog } from "./agent-memory-catalog";
import { projectMemoryCatalog, type MemoryFilter } from "./memory-catalog-model";

const snapshot: MemorySnapshot = { layout: "topic", truncated: false, documents: [{
  kind: "topic", indexed: true, modified_at: "2026-09-06", path: "memory/reference.md", size: 23,
  title: "reference.md", description: "跨区域项目资料 · Cross-region project reference", type: "reference",
}] };

function catalog({ empty = false, filter = "all", query = "", refreshing = false }: {
  empty?: boolean; filter?: MemoryFilter; query?: string; refreshing?: boolean;
} = {}) {
  const projected = projectMemoryCatalog(empty ? { layout: "empty", documents: [], truncated: false } : snapshot,
    "memory/reference.md", filter, query);
  const actions = { onFilterChange: vi.fn(), onQueryChange: vi.fn(), onRefresh: vi.fn(), onSelectDocument: vi.fn() };
  const result = render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}>
    <AgentMemoryCatalog {...projected} {...actions} filter={filter} query={query} refreshing={refreshing} />
  </I18N_CONTEXT.Provider>);
  return { ...result, actions };
}

it("shows one true empty state without an unrelated no-match or reset action", () => {
  catalog({ empty: true });
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText("No memory yet")).toBeTruthy();
  expect(screen.queryByText("No matching memory files")).toBeNull();
  expect(screen.queryByRole("button", { name: "Clear filters" })).toBeNull();
});

it("clears both query and type when their combined result is empty", async () => {
  const { actions } = catalog({ filter: "feedback", query: "missing" });
  expect(screen.getByText("No matching memory files")).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(actions.onQueryChange).toHaveBeenCalledExactlyOnceWith("");
  expect(actions.onFilterChange).toHaveBeenCalledExactlyOnceWith("all");
  expect(actions.onRefresh).not.toHaveBeenCalled();
});

it("names search and type separately and keeps selection available during refresh", async () => {
  const { actions } = catalog({ refreshing: true });
  const search = screen.getByRole("searchbox", { name: MESSAGES.en["capability.memory_search_placeholder"] });
  expect(search.getAttribute("placeholder")).toBe(MESSAGES.en["capability.memory_search_placeholder"]);
  expect(screen.getByRole("button", { name: "Filter memory type" })).toBeTruthy();
  expect((screen.getByRole("button", { name: "Refresh" }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: /跨区域项目资料/ }));
  expect(actions.onSelectDocument).toHaveBeenCalledExactlyOnceWith("memory/reference.md");
  expect(actions.onRefresh).not.toHaveBeenCalled();
});

it("keeps the full filename and exact path on a selected summarized row", () => {
  catalog();
  const row = screen.getByRole("button", { name: /跨区域项目资料/ });
  expect(row.getAttribute("title")).toBeNull();
  expect(row.getAttribute("aria-pressed")).toBe("true");
  expect(screen.getByText("reference.md")).toBeTruthy();
});
