// INPUT: Domain-approved settings recovery actions and feedback.
// OUTPUT: Direct DOM evidence for action routing, busy locks and success/no-action states.
// POS: Recovery notice tests; no preferences or Echo mutations are performed.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { EchoSettingsReliabilityNotice } from "./echo-settings-reliability-notice";
import { PreferencesReliabilityNotice } from "./preferences-reliability-notice";
const feedback = { tone: "warning", title: "Check the result", impact: "Previous settings remain visible" } as const;

it.each(["repair", "compare", "check"])("routes Preferences %s explicitly and blocks checking", async (mode) => {
  const recovery = { canCompare: mode !== "check", canRepairProjection: mode === "repair", checking: false, repairing: false,
    checkLatest: vi.fn(), reapplyDraft: vi.fn(), repairProjection: vi.fn() };
  const renderNotice = () => <I18nProvider><PreferencesReliabilityNotice feedback={feedback} recovery={recovery} /></I18nProvider>;
  const view = render(renderNotice());
  const expected = mode === "repair" ? recovery.repairProjection : mode === "compare" ? recovery.reapplyDraft : recovery.checkLatest;
  expect(expected).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button"));
  expect(expected).toHaveBeenCalledOnce();
  expect([recovery.checkLatest, recovery.reapplyDraft, recovery.repairProjection].filter(command => command.mock.calls.length)).toHaveLength(1);
  recovery.checking = true;
  view.rerender(renderNotice());
  await userEvent.click(screen.getByRole("button"));
  expect(expected).toHaveBeenCalledOnce();
});

it.each(["repair", "compare", "check"])("routes Echo %s explicitly and blocks checking", async (mode) => {
  const recovery = { canCheckLatest: true, canCompare: mode !== "check", canFinishDisabling: mode === "repair", checking: false, repairing: false,
    checkLatest: vi.fn(), reapplyChange: vi.fn(), finishDisabling: vi.fn() };
  const renderNotice = () => <I18nProvider><EchoSettingsReliabilityNotice feedback={feedback} recovery={recovery} /></I18nProvider>;
  const view = render(renderNotice());
  const expected = mode === "repair" ? recovery.finishDisabling : mode === "compare" ? recovery.reapplyChange : recovery.checkLatest;
  expect(expected).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole("button"));
  expect(expected).toHaveBeenCalledOnce();
  expect([recovery.checkLatest, recovery.reapplyChange, recovery.finishDisabling].filter(command => command.mock.calls.length)).toHaveLength(1);
  recovery.checking = true;
  view.rerender(renderNotice());
  await userEvent.click(screen.getByRole("button"));
  expect(expected).toHaveBeenCalledOnce();
});

it("shows no recovery action on success or when no recovery is authorized", () => {
  const recovery = { canCheckLatest: false, canCompare: false, canFinishDisabling: false, checking: false, repairing: false,
    checkLatest: vi.fn(), reapplyChange: vi.fn(), finishDisabling: vi.fn() };
  const view = render(<I18nProvider><EchoSettingsReliabilityNotice feedback={feedback} recovery={recovery} /><PreferencesReliabilityNotice feedback={feedback} /></I18nProvider>);
  expect(screen.queryByRole("button")).toBeNull();
  view.rerender(<I18nProvider><EchoSettingsReliabilityNotice feedback={{tone: "success", title: "Saved", message: "Done"}} recovery={{...recovery, canCheckLatest: true}} /></I18nProvider>);
  expect(screen.getByText("Saved")).toBeTruthy();
  expect(screen.queryByRole("button")).toBeNull();
});
