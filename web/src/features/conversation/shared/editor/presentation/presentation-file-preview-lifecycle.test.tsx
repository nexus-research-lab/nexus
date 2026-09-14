// INPUT: Deferred PPTX parsing, owner/file transitions and object URL resources.
// OUTPUT: Obsolete results are discarded and disposed without revoking the active presentation.
// POS: Offline view/controller lifecycle; parser and slide drawing are controlled fixtures.
import type { ReactNode } from "react";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { PresentationFilePreview } from "./presentation-file-preview";
import { parsePptx } from "./presentation-pptx-parser";
import type { PresentationSlide } from "./presentation-preview-model";

vi.mock("../office-preview-resource", () => ({ fetchOfficePreviewBuffer: vi.fn() }));
vi.mock("./presentation-pptx-parser", () => ({ parsePptx: vi.fn() }));
vi.mock("./presentation-slide-canvas", () => ({
  PresentationSlideCanvas: ({ slide, thumbnail }: { slide: PresentationSlide; thumbnail?: boolean }) => (
    <div>{thumbnail ? null : slide.title}</div>
  ),
}));

type ParsedPresentation = Awaited<ReturnType<typeof parsePptx>>;
function parsed(title: string): ParsedPresentation {
  return { objectUrls: [`blob:${title}`], slides: [{
    id: title, title, background: "#fff", elements: [], width: 1280, height: 720,
  }] };
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => { resolve = yes; });
  return { promise, resolve };
}
function localized(children: ReactNode) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>;
}
const file = { agentId: "agent", path: "slides.pptx", fileName: "slides.pptx", isPreviewFocused: false, onTogglePreviewFocus: vi.fn() };
const revoke = vi.fn();
beforeEach(() => {
  vi.mocked(fetchOfficePreviewBuffer).mockReset().mockResolvedValue(new ArrayBuffer(4));
  vi.mocked(parsePptx).mockReset();
  revoke.mockClear();
  vi.stubGlobal("URL", class extends URL { static revokeObjectURL = revoke; });
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

it("disposes a late parse after A to B to A without replacing the new A result", async () => {
  const old = deferred<ParsedPresentation>();
  vi.mocked(parsePptx).mockReturnValueOnce(old.promise)
    .mockResolvedValueOnce(parsed("B")).mockResolvedValueOnce(parsed("New A"));
  const view = render(localized(<PresentationFilePreview {...file} />));
  await waitFor(() => expect(parsePptx).toHaveBeenCalledTimes(1));
  view.rerender(localized(<PresentationFilePreview {...file} path="b.pptx" />));
  await screen.findByText("B");
  view.rerender(localized(<PresentationFilePreview {...file} />));
  await screen.findByText("New A");
  await act(async () => old.resolve(parsed("Old A")));
  expect(screen.queryByText("Old A")).toBeNull();
  expect(screen.getByText("New A")).toBeTruthy();
  expect(revoke.mock.calls).toEqual([["blob:B"], ["blob:Old A"]]);
  expect(vi.mocked(fetchOfficePreviewBuffer).mock.calls[0][0].signal.aborted).toBe(true);
  view.unmount();
  expect(revoke.mock.calls).toEqual([["blob:B"], ["blob:Old A"], ["blob:New A"]]);
});

it("disposes a result when the owner changes before publication and reloads after notification", async () => {
  const old = deferred<ParsedPresentation>();
  vi.mocked(parsePptx).mockReturnValueOnce(old.promise).mockResolvedValueOnce(parsed("Current owner"));
  const view = render(localized(<PresentationFilePreview {...file} />));
  await waitFor(() => expect(parsePptx).toHaveBeenCalledTimes(1));
  advanceAuthOwnerScopeGeneration();
  await act(async () => old.resolve(parsed("Previous owner")));
  expect(screen.queryByText("Previous owner")).toBeNull();
  expect(revoke).toHaveBeenCalledWith("blob:Previous owner");
  act(() => publishAuthOwnerScopeGeneration());
  await screen.findByText("Current owner");
  expect(fetchOfficePreviewBuffer).toHaveBeenCalledTimes(2);
  expect(revoke).not.toHaveBeenCalledWith("blob:Current owner");
  view.unmount();
  expect(revoke).toHaveBeenCalledWith("blob:Current owner");
});

it("offers an explicit retry after parser failure and disposes a parse completed after unmount", async () => {
  const pending = deferred<ParsedPresentation>();
  vi.mocked(parsePptx).mockRejectedValueOnce(new Error("invalid archive")).mockReturnValueOnce(pending.promise);
  const view = render(localized(<PresentationFilePreview {...file} />));
  const retry = await screen.findByRole("button", { name: "workspace_file.retry_preview" });
  await userEvent.setup().click(retry);
  await waitFor(() => expect(parsePptx).toHaveBeenCalledTimes(2));
  view.unmount();
  expect(vi.mocked(fetchOfficePreviewBuffer).mock.calls[1][0].signal.aborted).toBe(true);
  await act(async () => pending.resolve(parsed("Unmounted")));
  expect(revoke.mock.calls).toEqual([["blob:Unmounted"]]);
});
