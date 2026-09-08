// INPUT: Skill identity, lock/busy state and an explicit enable/disable callback.
// OUTPUT: Only an available switch emits the exact Skill command; the shared card remains non-interactive.
// POS: Agent Skill presentation regression; resource and mutation lifecycles are tested by their owners.

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { AgentSkillEntry } from "@/types/capability/skill";
import { AgentSkillCard } from "./agent-skill-card";

const skill: AgentSkillEntry = {
  name: "custom-review", title: "Review skill", description: "Check a proposed change.", scope: "any",
  tags: [], category_key: "development", category_name: "Development", source_type: "external",
  source_ref: "", version: "1", enabled_for_agent: false, locked: false, has_update: false, deletable: true,
};

describe("AgentSkillCard", () => {
  it("preserves exact switch commands without making the whole card clickable", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    render(<I18nProvider><AgentSkillCard actionLabel="Enable" blocked={false} busy={false} commandBusy={false} onAction={onAction} skill={skill} /></I18nProvider>);
    await user.click(screen.getByText("Review skill"));
    expect(onAction).not.toHaveBeenCalled();
    expect(screen.getByRole("article").hasAttribute("tabindex")).toBe(false);
    await user.click(screen.getByRole("switch", { name: "Enable Review skill" }));
    expect(onAction).toHaveBeenCalledExactlyOnceWith(skill);
  });

  it("keeps blocked and pending commands inert and locked Skills without a switch", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const view = (blocked: boolean, commandBusy: boolean, locked = false) => <I18nProvider>
      <AgentSkillCard actionLabel="Enable" blocked={blocked} busy={commandBusy} commandBusy={commandBusy} onAction={onAction} skill={{ ...skill, locked }} />
    </I18nProvider>;
    const { rerender } = render(view(true, false));
    await user.click(screen.getByRole("switch"));
    rerender(view(false, true));
    const pending = screen.getByRole("switch") as HTMLButtonElement;
    expect(pending.disabled).toBe(true);
    await user.click(pending);
    expect(onAction).not.toHaveBeenCalled();
    rerender(view(false, false, true));
    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.getByText("Review skill")).toBeTruthy();
  });
});
