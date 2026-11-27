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
});
