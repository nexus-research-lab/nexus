// INPUT: Real schedule forms with interval/monthly/cron drafts and isolated commands.
// OUTPUT: Instance-scoped fields, named interval controls, readable help and exact selection callbacks.
// POS: Schedule view regression; clock/calendar and mutation state machines retain their own tests.

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef, type ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { TaskSchedulePanel, TaskScheduleAdvanced } from "./task-schedule-panel";
import type { ScheduleKind } from "../scheduled-task-dialog-types";

function makeProps(kind: ScheduleKind): ComponentProps<typeof TaskSchedulePanel> {
  const parts = { hour12: "09", meridiem: "am" as const, minute: "30", second: "00" };
  return {
    actions: {
      closeDailyPicker: vi.fn(), closeSinglePicker: vi.fn(), goToNextMonth: vi.fn(), goToPrevMonth: vi.fn(),
      isSingleDateDisabled: () => false, isSingleHourDisabled: () => false, isSingleMeridiemDisabled: () => false,
      isSingleMinuteDisabled: () => false, isSingleSecondDisabled: () => false,
      setCronExpression: vi.fn(), setEveryUnit: vi.fn(), setEveryValue: vi.fn(), setKind: vi.fn(), setMonthlyDay: vi.fn(),
      setTimezone: vi.fn(), toggleDailyPicker: vi.fn(), toggleSinglePicker: vi.fn(), toggleWeekday: vi.fn(),
      updateDailyPicker: vi.fn(), updateSinglePicker: vi.fn(),
    },
    formError: null, form: { enabled: true, executionKind: "agent", instruction: "Keep this exact instruction" },
    formActions: { setEnabled: vi.fn(), setInstruction: vi.fn() }, isReconciling: false,
    isRestoredCreateIntent: false, isMutationReviewed: false, mutationFailure: null,
    onConfirmMutationReviewed: vi.fn(), onReconcile: vi.fn(), onStartNewCreateIntent: vi.fn(),
    refs: { dailyPickerAnchorRef: createRef<HTMLButtonElement>(), singlePickerAnchorRef: createRef<HTMLButtonElement>() },
    schedule: { cronExpression: "0 9 15 * *", dailyTime: "09:30", everyUnit: "hours", everyValue: "2", kind,
      monthlyDay: "15", runAt: "2030-01-15T09:30", selectedWeekdays: ["mo"], timezone: "UTC" },
    view: { dailyDisplay: "09:30", dailyMeridiemParts: parts, isDailyPickerOpen: false, isSinglePickerOpen: false,
      runAtDisplay: "2030-01-15 09:30", runAtParts: { date: "2030-01-15" }, singleMeridiemParts: parts,
      singlePickerDays: [], singlePickerMonth: "2030-01" },
  };
}

function TestForm({ props, label }: { props: ComponentProps<typeof TaskSchedulePanel>; label: string }) {
  return <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <section aria-label={label}><TaskSchedulePanel {...props} /><TaskScheduleAdvanced {...props} /></section>
  </I18N_CONTEXT.Provider>;
}

describe("TaskSchedulePanel", () => {
  it("names the interval value and retains independent value, unit and instruction commands", async () => {
    const user = userEvent.setup();
    const props = makeProps("every");
    render(<TestForm props={props} label="Interval task" />);
    const amount = screen.getByRole("spinbutton", { name: "capability.scheduled_dialog_every" });
    fireEvent.change(amount, { target: { value: "007" } });
    expect(props.actions.setEveryValue).toHaveBeenCalledExactlyOnceWith("007");
    await user.click(screen.getByRole("button", { name: "capability.scheduled_dialog_select_interval_unit" }));
    await user.click(screen.getByRole("option", { name: /minutes/ }));
    expect(props.actions.setEveryUnit).toHaveBeenCalledExactlyOnceWith("minutes");
    fireEvent.change(screen.getByRole("textbox", { name: "capability.scheduled_dialog_instruction" }), { target: { value: "  Keep\nnew lines  " } });
    expect(props.formActions.setInstruction).toHaveBeenCalledExactlyOnceWith("  Keep\nnew lines  ");
    expect(props.actions.setKind).not.toHaveBeenCalled();
  });

  it.each(["every", "monthly", "custom", "cron"] as const)("keeps %s field labels and schedule groups isolated between two forms", (kind) => {
    const { container } = render(<><TestForm props={makeProps(kind)} label="First" /><TestForm props={makeProps(kind)} label="Second" /></>);
    for (const label of container.querySelectorAll<HTMLLabelElement>("label[for]")) {
      expect(label.control).not.toBeNull();
      expect(label.control?.closest("section[aria-label]")).toBe(label.closest("section[aria-label]"));
    }
    const ids = [...container.querySelectorAll("[id]")].map((element) => element.id);
    expect(new Set(ids).size).toBe(ids.length);
    const second = within(screen.getByRole("region", { name: "Second" }));
    expect(second.getAllByRole("button", { name: "capability.scheduled_dialog_schedule" })).toHaveLength(1);
    if (kind === "monthly" || kind === "custom") {
      const label = `capability.scheduled_dialog_${kind === "monthly" ? "monthly_day" : "custom_cron"}`;
      const control = second.getByRole(kind === "monthly" ? "spinbutton" : "textbox", { name: label });
      expect(document.getElementById(control.getAttribute("aria-describedby")!)?.textContent).toBe(`${label}_help`);
    }
  });

  it("names weekday help and changes only the exact selected day or enabled flag", async () => {
    const user = userEvent.setup();
    const props = makeProps("cron");
    render(<TestForm props={props} label="Weekly task" />);
    const days = screen.getByRole("group", { name: "capability.scheduled_dialog_execution_days" });
    expect(document.getElementById(days.getAttribute("aria-describedby")!)?.textContent).toBe("capability.scheduled_dialog_execution_days_help");
    await user.click(within(days).getByRole("button", { name: "capability.scheduled_dialog_weekday_wed" }));
    expect(props.actions.toggleWeekday).toHaveBeenCalledExactlyOnceWith("we");
    await user.click(screen.getByRole("checkbox", { name: "capability.scheduled_dialog_enabled" }));
    expect(props.formActions.setEnabled).toHaveBeenCalledExactlyOnceWith(false);
    expect(props.actions.setKind).not.toHaveBeenCalled();
  });
});
