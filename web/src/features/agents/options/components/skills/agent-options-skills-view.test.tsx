// INPUT: Skill 初始加载、读取失败和已有列表快照。
// OUTPUT: 加载具名，读取失败不伪造空目录，已有 Skill 保留但开关禁用。
// POS: 技能页资源状态投影回归。
import { render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { AgentOptionsSkillsView } from "./agent-options-skills-view";
const base: ComponentProps<typeof AgentOptionsSkillsView> = {
  agentId: "agent", busySkillName: null, blockedSkillNames: new Set(), cancelDisable: vi.fn(), commandBusy: false,
  confirmDisable: vi.fn(), loading: false, mutationFailures: [], pendingDisableSkill: null,
  projection: { available: [], availableEmptyState: "catalog_empty", enabled: [], visibleAvailable: [] },
  readFailure: null, refresh: vi.fn(async () => undefined), requestSkillAction: vi.fn(), searchQuery: "", setSearchQuery: vi.fn(),
};
const view = (props: Partial<typeof base>) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}><AgentOptionsSkillsView {...base} {...props} /></I18N_CONTEXT.Provider>;
it("distinguishes loading, failed reads and confirmed empty results", () => {
  const { rerender } = render(view({ loading: true }));
  expect(screen.getByRole("status").textContent).toContain("common.loading");
  rerender(view({ readFailure: { title: "Read failed", impact: "Try again" } }));
  expect(screen.getByText("Read failed")).toBeTruthy();
  expect(screen.queryByText("agent_options.skills.empty_enabled")).toBeNull();
  expect(screen.queryByText("agent_options.skills.empty_available")).toBeNull();
  rerender(view({}));
  expect(screen.getByText("agent_options.skills.empty_enabled")).toBeTruthy();
  expect(screen.getByText("agent_options.skills.empty_available")).toBeTruthy();
});
it("preserves existing cards during read failure while blocking unsafe actions", () => {
  const skill = { name: "review", title: "Review", description: "Review changes", scope: "any", tags: [], category_key: "dev", category_name: "Dev", source_type: "external", source_ref: "", version: "1", enabled_for_agent: true, locked: false, has_update: false, deletable: true } as const;
  render(view({ projection: { ...base.projection, enabled: [{ ...skill, tags: [] }] }, readFailure: { title: "Read failed", impact: "Snapshot may be stale" } }));
  expect(screen.getByText("Review")).toBeTruthy();
  expect((screen.getByRole("switch") as HTMLButtonElement).disabled).toBe(true);
});
