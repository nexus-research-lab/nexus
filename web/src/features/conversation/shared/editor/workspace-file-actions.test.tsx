// INPUT: Explicit file actions, controlled completion order, locale and owner changes.
// OUTPUT: Only the current file's latest action can publish localized failure feedback.
// POS: Offline command/feedback regression; downloads and native calls are mocked.
import { StrictMode, type ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
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

  it.each([false, true])("keeps exact file arguments and the desktop=%s action name", async (desktop) => {
    vi.mocked(isDesktopRuntime).mockReturnValue(desktop);
    render(localized(<ActionButton {...file} />));
    const key = desktop ? "workspace_file.reveal_named" : "workspace_file.download_named";
    const button = screen.getByRole("button", { name: MESSAGES.en[key].replace("{name}", file.fileName) });
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
    await act(async () => { fireEvent.click(button); });
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith(file.agentId, file.path, file.fileName);
  });

  it("does not replace a newer successful action with an older failure", async () => {
    const old = pendingAction();
    const latest = pendingAction();
    render(localized(<ActionButton {...file} />));
    clickAction(); clickAction();
    await act(async () => { latest.resolve(); });
    await fail(old);
    expect(screen.queryByRole("status")).toBeNull();
    expect(downloadWorkspaceFileApi).toHaveBeenCalledTimes(2);
  });

  it("keeps the latest failure localized across language changes and clears it on the next explicit attempt", async () => {
    const first = pendingAction();
    const view = render(localized(<ActionButton {...file} />));
    clickAction();
    view.rerender(localized(<ActionButton {...file} />, "zh"));
    await fail(first);
    expect(screen.getByText(MESSAGES.zh["workspace_file.external_action_failed"])).toBeTruthy();
    view.rerender(localized(<ActionButton {...file} />));
    expect(screen.getByText(failureTitle)).toBeTruthy();
    expect(screen.queryByText("native diagnostic")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: MESSAGES.en["common.close"] }));
    expect(screen.queryByRole("status")).toBeNull();
    const second = pendingAction();
    clickAction(); await fail(second);
    expect(screen.getByText(failureTitle)).toBeTruthy();
    await act(async () => { clickAction(); });
    expect(screen.queryByRole("status")).toBeNull();
    expect(downloadWorkspaceFileApi).toHaveBeenCalledTimes(3);
  });

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

  it("fences owner changes before notification, then resets existing feedback on publication", async () => {
    const first = pendingAction();
    render(localized(<ActionButton {...file} />));
    clickAction();
    advanceAuthOwnerScopeGeneration();
    await fail(first);
    expect(screen.queryByRole("status")).toBeNull();
    clickAction();
    expect(downloadWorkspaceFileApi).toHaveBeenCalledTimes(1);
    act(() => publishAuthOwnerScopeGeneration());
    const current = pendingAction();
    clickAction(); await fail(current);
    expect(screen.getByText(failureTitle)).toBeTruthy();
    act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("discards a pending failure after unmount without repeating the operation", async () => {
    const pending = pendingAction();
    const view = render(localized(<ActionButton {...file} />));
    clickAction(); view.unmount();
    await fail(pending);
    expect(console.error).not.toHaveBeenCalled();
    expect(downloadWorkspaceFileApi).toHaveBeenCalledTimes(1);
  });

});
