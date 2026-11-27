// INPUT: Controlled DOCX reads/parser results and the real preview controller/view lifecycle.
// OUTPUT: Failed reads retry through stable hosts; stale file/owner results and measurements cannot commit.
// POS: Offline DOCX integration; parser fixtures replace document decoding, not the React hosts.
import type { ReactNode } from "react";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { renderAsync } from "docx-preview";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { fetchOfficePreviewBuffer } from "../office-preview-resource";
import { DocumentFilePreview } from "./document-file-preview";

vi.mock("../office-preview-resource", () => ({ fetchOfficePreviewBuffer: vi.fn() }));
vi.mock("docx-preview", () => ({ renderAsync: vi.fn() }));
const renderFixture: typeof renderAsync = async (_buffer, container, styles) => {
  const page = document.createElement("section");
  page.className = "nexus-docx-preview";
  page.style.width = "600px";
  page.textContent = "Parsed document";
  container.append(page);
  const style = document.createElement("style");
  style.textContent = ".document-fixture { color: black; }";
  styles?.append(style);
};
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}
function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}
const file = { agentId: "document-agent", path: "output/report.docx", fileName: "report.docx", isPreviewFocused: false, onTogglePreviewFocus: vi.fn() };
beforeEach(() => {
  vi.mocked(fetchOfficePreviewBuffer).mockReset();
  vi.mocked(renderAsync).mockReset().mockImplementation(renderFixture);
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("retries a failed read through the same live render and style hosts", async () => {
  vi.mocked(fetchOfficePreviewBuffer).mockRejectedValueOnce(new Error("read failed"))
    .mockResolvedValueOnce(new ArrayBuffer(4));
  const view = render(localized(<DocumentFilePreview {...file} />));
  const host = view.container.querySelector(".nexus-docx-preview-host");
  const styleHost = view.container.querySelector(".contents[aria-hidden]");
  await screen.findByText(MESSAGES.en["workspace_file.document_preview_failed"]);
  expect(view.container.querySelector(".nexus-docx-preview-host")).toBe(host);
  expect(host?.hasAttribute("inert")).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["workspace_file.retry_preview"] }));
  await waitFor(() => expect(renderAsync).toHaveBeenCalledTimes(1));
  expect(view.container.querySelector(".nexus-docx-preview-host")).toBe(host);
  expect(view.container.querySelector(".contents[aria-hidden]")).toBe(styleHost);
  expect(host?.hasAttribute("inert")).toBe(false);
  expect(screen.getByText("Parsed document")).toBeTruthy();
  expect(screen.queryByRole("status")).toBeNull();
  expect(fetchOfficePreviewBuffer).toHaveBeenCalledTimes(2);
});

it.each(["read", "parse"] as const)("fences the old owner's %s completion before publication", async (phase) => {
  const read = deferred<ArrayBuffer>();
  const parse = deferred<void>();
  vi.mocked(fetchOfficePreviewBuffer).mockResolvedValue(new ArrayBuffer(4));
  if (phase === "read") vi.mocked(fetchOfficePreviewBuffer).mockReturnValueOnce(read.promise);
  else vi.mocked(renderAsync).mockImplementationOnce(async (...args) => { await renderFixture(...args); await parse.promise; });
  render(localized(<DocumentFilePreview {...file} />));
  if (phase === "parse") await waitFor(() => expect(renderAsync).toHaveBeenCalledTimes(1));
  advanceAuthOwnerScopeGeneration();
  await act(async () => { if (phase === "read") read.resolve(new ArrayBuffer(4)); else parse.resolve(); });
  expect(screen.queryByText("Parsed document")).toBeNull();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  act(() => publishAuthOwnerScopeGeneration());
  await screen.findByText("Parsed document");
  expect(fetchOfficePreviewBuffer).toHaveBeenCalledTimes(2);
});
