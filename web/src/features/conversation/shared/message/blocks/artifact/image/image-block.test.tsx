// INPUT: Deferred image requests completing after unmount or replacement.
// OUTPUT: Cancelled responses never allocate leaked Blob URLs; live URLs are revoked.
// POS: Image artifact resource lifecycle regression.
import { act, render } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { getSessionMessageImageDetailApi } from "@/lib/api/conversation/session-api";
import { ImageBlock } from "./image-block";
vi.mock("@/lib/api/conversation/session-api", () => ({ getSessionMessageImageDetailApi: vi.fn() }));
vi.mock("@/hooks/agent/use-workspace-markdown", () => ({ useWorkspaceMarkdown: () => ({ currentAgentId: null, resolveFilePath: () => null }) }));
it("does not allocate a Blob URL for a cancelled detail request", async () => {
  let resolve!: (blob: Blob) => void;
  vi.mocked(getSessionMessageImageDetailApi).mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  const create = vi.fn(() => "blob:detail");
  const old = URL.createObjectURL;
  URL.createObjectURL = create;
  try {
    const { unmount } = render(<I18nProvider><ImageBlock block={{ type: "image", detail_ref: "image-ref", detail_session_key: "session" }} /></I18nProvider>);
    unmount();
    await act(async () => resolve(new Blob(["image"])));
    expect(create).not.toHaveBeenCalled();
  } finally { URL.createObjectURL = old; }
});
