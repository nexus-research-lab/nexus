// INPUT: Local file replacements, deferred bounded reads and simultaneous dialog instances.
// OUTPUT: Preview state stays with its file and every modal owns its accessible title.
// POS: Offline attachment preview regression; native image/text decoding and visual layout are not exercised.
import type { ReactNode } from "react";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ComposerAttachmentPreviewDialog } from "./composer-attachment-preview-dialog";
import type { ComposerLocalAttachment } from "./composer-local-attachment-model";

let createUrl: ReturnType<typeof vi.fn>;
let revokeUrl: ReturnType<typeof vi.fn>;
beforeEach(() => {
  createUrl = vi.fn().mockImplementation(() => `blob:file-${createUrl.mock.calls.length}`);
  revokeUrl = vi.fn();
  vi.stubGlobal("URL", class extends URL { static createObjectURL = createUrl; static revokeObjectURL = revokeUrl; });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });
function localized(children: ReactNode) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>;
}
const attachment = (name: string, kind: "image" | "text" = "image"): ComposerLocalAttachment => ({ id: name, kind, file: new File([name], name) });
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

it("clears a failed image when the selected file is replaced and releases both URLs", async () => {
  const first = attachment("broken.png"); const next = attachment("next.png");
  const view = render(localized(<ComposerAttachmentPreviewDialog attachment={first} onClose={vi.fn()} />));
  fireEvent.error(await screen.findByRole("img", { name: "broken.png" }));
  expect(screen.getByText("composer.attachment_preview_failed")).toBeTruthy();
  view.rerender(localized(<ComposerAttachmentPreviewDialog attachment={next} onClose={vi.fn()} />));
  expect(await screen.findByRole("img", { name: "next.png" })).toBeTruthy();
  expect(screen.queryByText("composer.attachment_preview_failed")).toBeNull();
  expect(revokeUrl).toHaveBeenCalledWith("blob:file-1");
  view.unmount();
  expect(revokeUrl.mock.calls).toEqual([["blob:file-1"], ["blob:file-2"]]);
});

it("gives simultaneous previews distinct titles owned by their dialog instances", () => {
  render(localized(<>
    <ComposerAttachmentPreviewDialog attachment={attachment("first.png")} onClose={vi.fn()} />
    <ComposerAttachmentPreviewDialog attachment={attachment("second.png")} onClose={vi.fn()} />
  </>));
  const dialogs = [...document.querySelectorAll('[role="dialog"]')];
  const ids = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
  expect(new Set(ids).size).toBe(2);
  expect(ids.map((id) => document.getElementById(id!)?.textContent)).toEqual(["first.png", "second.png"]);
});

it.each(["resolve", "reject"] as const)("bounds the read and ignores an old text %s after switching files", async (outcome) => {
  const oldRead = deferred<string>();
  const first = attachment("old.txt", "text");
  const next: ComposerLocalAttachment = { id: "large", kind: "text", file: new File([new Uint8Array(600 * 1024)], "large.txt") };
  vi.spyOn(first.file, "slice").mockReturnValue({ text: () => oldRead.promise } as Blob);
  const slice = vi.spyOn(next.file, "slice").mockReturnValue({ text: async () => "Current text" } as Blob);
  const view = render(localized(<ComposerAttachmentPreviewDialog attachment={first} onClose={vi.fn()} />));
  view.rerender(localized(<ComposerAttachmentPreviewDialog attachment={next} onClose={vi.fn()} />));
  await screen.findByText("Current text");
  expect(slice).toHaveBeenCalledExactlyOnceWith(0, 512 * 1024);
  expect(screen.getByText("composer.text_preview_truncated")).toBeTruthy();
  await act(async () => { if (outcome === "resolve") oldRead.resolve("Old text"); else oldRead.reject(new Error("old")); });
  expect(screen.queryByText("Old text")).toBeNull();
  expect(screen.queryByText("composer.attachment_preview_failed")).toBeNull();
  const region = screen.getByRole("region", { name: "large.txt" });
  expect(region.tabIndex).toBe(0);
  region.focus();
  await waitFor(() => expect(document.activeElement).toBe(region));
});
