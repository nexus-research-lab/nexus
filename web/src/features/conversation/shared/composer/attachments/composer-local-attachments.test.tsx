// INPUT: Local draft files, exact removal identities and Session reset boundaries.
// OUTPUT: Independent keyboard preview/removal commands and deterministic Object URL cleanup.
// POS: Composer attachment integration regression; real text decoding and geometry use the browser fixture.

import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { ComposerAttachmentList } from "./composer-local-attachments";
import type { ComposerLocalAttachment } from "./composer-local-attachment-model";

const attachments: ComposerLocalAttachment[] = [
  { id: "image-id", kind: "image", file: new File(["image"], "preview.png", { type: "image/png" }) },
  { id: "file-id", kind: "file", file: new File(["archive"], "archive.zip", { type: "application/zip" }) },
];

let createUrl: ReturnType<typeof vi.fn>;
let revokeUrl: ReturnType<typeof vi.fn>;
beforeEach(() => {
  createUrl = vi.fn().mockImplementation(() => `blob:preview-${createUrl.mock.calls.length}`);
  revokeUrl = vi.fn();
  vi.stubGlobal("URL", class extends URL {
    static createObjectURL = createUrl;
    static revokeObjectURL = revokeUrl;
  });
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function View({ files = attachments, reset = "first", onRemove = vi.fn(), onSubmit = vi.fn() }) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key, params) => params?.name ? `${key}:${params.name}` : key }}>
    <form onSubmit={onSubmit}>
      <ComposerAttachmentList attachments={files} onRemove={onRemove} previewResetKey={reset} removeLabel="Remove attachment" />
    </form>
  </I18N_CONTEXT.Provider>;
}

describe("ComposerAttachmentList", () => {
  it("keeps preview and exact removal independent and never submits the surrounding form", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const onSubmit = vi.fn();
    render(<View onRemove={onRemove} onSubmit={onSubmit} />);
    await user.click(screen.getByText("archive.zip"));
    expect(onRemove).not.toHaveBeenCalled();
    const preview = screen.getByRole("button", { name: "composer.preview_image:preview.png" });
    await user.tab();
    // Start explicitly at the preview; native Enter is the activation boundary.
    preview.focus();
    await user.keyboard("{Enter}");
    expect(await screen.findByRole("dialog", { name: "preview.png" })).toBeTruthy();
    expect(onRemove).not.toHaveBeenCalled();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(preview));
    await user.click(screen.getAllByRole("button", { name: "Remove attachment" })[0]);
    expect(onRemove).toHaveBeenCalledExactlyOnceWith("image-id");
    expect(screen.queryByRole("dialog")).toBeNull();
    await user.click(screen.getAllByRole("button", { name: "Remove attachment" })[1]);
    expect(onRemove).toHaveBeenLastCalledWith("file-id");
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("closes only the transient preview on Session change and releases each Object URL", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const { rerender, unmount } = render(<View onRemove={onRemove} />);
    await user.click(screen.getByRole("button", { name: "composer.preview_image:preview.png" }));
    expect(await screen.findByRole("dialog")).toBeTruthy();
    expect(createUrl).toHaveBeenCalledTimes(2);
    rerender(<View onRemove={onRemove} reset="next" />);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: "composer.preview_image:preview.png" })).toBeTruthy();
    expect(screen.getByText("archive.zip")).toBeTruthy();
    expect(revokeUrl).toHaveBeenCalledExactlyOnceWith("blob:preview-2");
    expect(onRemove).not.toHaveBeenCalled();
    unmount();
    expect(revokeUrl.mock.calls.map(([url]) => url).sort()).toEqual(["blob:preview-1", "blob:preview-2"]);
  });
});
