// INPUT: Explicit owner, candidate and session changes with current language.
// OUTPUT: Shared select semantics, identity-safe labels and obsolete-menu dismissal.
// POS: Room Goal owner-control DOM regressions; candidate policy stays in the model.
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { goalTestI18n } from "@/features/conversation/shared/goal/goal.test-support";
import { ROOM_GOAL_MEMBERS } from "../../room-goal.test-support";
import { RoomGoalLeadControl } from "./room-goal-lead-control";

type Props = Parameters<typeof RoomGoalLeadControl>[0];
function view(props: Partial<Props> = {}, locale: "en" | "zh" = "en") {
  return <I18N_CONTEXT.Provider value={goalTestI18n(locale)}>
    <RoomGoalLeadControl agentId="alpha" roomMembers={ROOM_GOAL_MEMBERS} scopeKey="session-1"
      disabled={false} onChange={vi.fn()} {...props} />
  </I18N_CONTEXT.Provider>;
}

describe("Room Goal owner control", () => {
  it.each(["en", "zh"] as const)("distinguishes duplicate and empty names without exposing IDs in %s", async (locale) => {
    const user = userEvent.setup(); const onChange = vi.fn();
    const members = [
      { ...ROOM_GOAL_MEMBERS[0], name: "Nova" }, { ...ROOM_GOAL_MEMBERS[1], name: "Nova" },
      { ...ROOM_GOAL_MEMBERS[0], agent_id: "internal-unnamed", name: " " },
    ];
    const { container } = render(view({ roomMembers: members, onChange }, locale));
    expect(container.querySelector("select")).toBeNull();
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("aria-label")).toContain("1 · Nova");
    await user.click(trigger);
    expect(screen.getByRole("option", { name: "2 · Nova" })).toBeTruthy();
    const name = locale === "en" ? "1 · Agent" : "1 · 智能体";
    expect(document.body.textContent).not.toContain("internal-unnamed");
    await user.click(screen.getByRole("option", { name }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("internal-unnamed");
    expect(document.activeElement).toBe(trigger);
  });
  it("keeps a missing owner visible as unavailable and requires an explicit new choice", async () => {
    const user = userEvent.setup(); const onChange = vi.fn();
    render(view({ agentId: "removed-owner", onChange }));
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("aria-label")).toContain(goalTestI18n("en").t("agent.selection_unavailable"));
    await user.click(trigger);
    const missing = screen.getByRole("option", { name: goalTestI18n("en").t("agent.selection_unavailable") });
    expect((missing as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(missing); expect(onChange).not.toHaveBeenCalled();
    await user.click(screen.getByRole("option", { name: "Beta" }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("beta");
  });
  it("preserves the menu through labels/order changes but consumes it on candidate or session changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(view()); const trigger = screen.getByRole("button");
    await user.click(trigger);
    rerender(view({ roomMembers: [...ROOM_GOAL_MEMBERS].reverse() }, "zh"));
    expect(screen.getByRole("listbox")).toBeTruthy();
    rerender(view({ roomMembers: ROOM_GOAL_MEMBERS.slice(0, 1) }));
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(screen.getByRole("button")).toBe(trigger);
    rerender(view()); expect(screen.queryByRole("listbox")).toBeNull();
    await user.click(trigger); rerender(view({ scopeKey: "session-2" }));
    expect(screen.queryByRole("listbox")).toBeNull();
    rerender(view()); expect(screen.queryByRole("listbox")).toBeNull();
  });
  it("allows clearing by explicit choice and disables an empty directory or busy control", async () => {
    const user = userEvent.setup(); const onChange = vi.fn();
    const { rerender } = render(view({ onChange }));
    await user.click(screen.getByRole("button"));
    await user.click(screen.getByRole("option", { name: "Owner" }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("");
    rerender(view({ roomMembers: [], onChange }));
    expect((screen.getByRole("button") as HTMLButtonElement).disabled).toBe(true);
    rerender(view({ disabled: true, onChange }));
    fireEvent.keyDown(screen.getByRole("button"), { key: "ArrowDown" });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("listbox")).toBeNull();
  });
});
