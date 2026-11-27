// INPUT: Explicit file actions, controlled completion order, locale and owner changes.
// OUTPUT: Only the current file's latest action can publish localized failure feedback.
// POS: Offline command/feedback regression; downloads and native calls are mocked.
import { StrictMode, type ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { WorkspaceFileDownloadButton } from "./workspace-file-preview-chrome";
import { WorkspaceArtifactExternalActionButton } from "../message/blocks/artifact/workspace-artifact-external-action";

vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: vi.fn(() => false) }));
vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn(async () => undefined) }));

function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <StrictMode><I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider></StrictMode>;
}

const file = { agentId: "agent-a", path: "output/报告.txt", fileName: "报告.txt" };
const failureTitle = MESSAGES.en["workspace_file.external_action_failed"];
function pendingAction() {
  let resolve!: () => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<void>((done, fail) => { resolve = done; reject = fail; });
  vi.mocked(downloadWorkspaceFileApi).mockReturnValueOnce(promise);
  return { resolve, reject };
}
async function fail(result: ReturnType<typeof pendingAction>) {
  await act(async () => { result.reject(new Error("native diagnostic")); });
}
function clickAction() {
  fireEvent.click(screen.getByRole("button", { name: /报告.txt/ }));
}

beforeEach(() => {
  vi.mocked(downloadWorkspaceFileApi).mockReset().mockResolvedValue(undefined);
  vi.mocked(isDesktopRuntime).mockReturnValue(false);
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});
afterEach(() => vi.restoreAllMocks());

describe.each(["preview", "artifact"] as const)("%s file action", (surface) => {
  const ActionButton = (target: typeof file) => surface === "preview"
    ? <WorkspaceFileDownloadButton {...target} />
    : <WorkspaceArtifactExternalActionButton action={target} />;

  it.each([
    { agentId: "agent-b" },
    { path: "other/报告.txt" },
    { fileName: "renamed.txt" },
  ])("resets feedback and rejects late failures after file scope changes: %j", async (change) => {
    const first = pendingAction();
    const view = render(localized(<ActionButton {...file} />));
    clickAction(); await fail(first);
    expect(screen.getByText(failureTitle)).toBeTruthy();
    view.rerender(localized(<ActionButton {...file} {...change} />));
    expect(screen.queryByRole("status")).toBeNull();
    view.rerender(localized(<ActionButton {...file} />));
    const late = pendingAction();
    clickAction();
    view.rerender(localized(<ActionButton {...file} {...change} />));
    view.rerender(localized(<ActionButton {...file} />));
    await fail(late);
    expect(screen.queryByRole("status")).toBeNull();
  });

});
