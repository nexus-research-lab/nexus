// INPUT: Native media events, Office fallback/view states and localized file actions.
// OUTPUT: Retry preserves exact file commands and loading/error feedback has one owner.
// POS: Offline preview behavior regressions; no renderer or browser visual claims.

import { createRef, type ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";

import { isDesktopRuntime } from "@/config/desktop-runtime";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { DocumentPreviewView } from "./document/document-preview-view";
import { BinaryFilePlaceholder, ImagePreview, PdfPreview } from "./media/media-file-preview";
import { OfficePreviewFallback } from "./office-preview-fallbacks";
import { TextFileContent } from "./text/text-file-content";

vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: vi.fn(() => false) }));
vi.mock("@/lib/api/agent/agent-api", () => ({
  downloadWorkspaceFileApi: vi.fn(async () => undefined),
  getWorkspaceFilePreviewUrl: (agent: string, path: string) => `/preview/${agent}?path=${encodeURIComponent(path)}`,
}));

function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: () => undefined, t }}>{children}</I18N_CONTEXT.Provider>;
}

const file = { agentId: "file-owner", path: "output/report.bin", fileName: "report.bin", isPreviewFocused: false, onTogglePreviewFocus: vi.fn() };
beforeEach(() => { vi.clearAllMocks(); vi.mocked(isDesktopRuntime).mockReturnValue(false); });

it.each(["document", "presentation", "spreadsheet"] as const)("keeps %s fallback actions available with one loading announcement", async (kind) => {
  render(localized(<OfficePreviewFallback {...file} kind={kind} />));
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getAllByText("Preparing preview")).toHaveLength(1);
  const buttons = screen.getAllByRole("button");
  await userEvent.click(buttons[0]);
  await userEvent.click(buttons[1]);
  expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith(file.agentId, file.path, file.fileName);
  expect(file.onTogglePreviewFocus).toHaveBeenCalledTimes(1);
});

it("keeps the native PDF sandbox and clears its loading feedback on frame load", () => {
  const { container } = render(localized(<PdfPreview {...file} />));
  const frame = container.querySelector("iframe")!;
  expect(frame.getAttribute("src")).toBe("/preview/file-owner?path=output%2Freport.bin");
  expect(frame.getAttribute("sandbox")).toBe("allow-downloads allow-same-origin");
  expect(screen.getAllByText("Preparing preview")).toHaveLength(1);
  fireEvent.load(frame);
  expect(screen.queryByRole("status")).toBeNull();
});

it("retries a failed image with a new native element and clears feedback on load", async () => {
  const { container } = render(localized(<ImagePreview {...file} />));
  const original = container.querySelector("img")!;
  expect(original.getAttribute("src")).toBe("/preview/file-owner?path=output%2Freport.bin");
  expect(screen.getAllByText("Preparing preview")).toHaveLength(1);
  fireEvent.error(original);
  expect(screen.queryByText("Preparing preview")).toBeNull();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["workspace_file.retry_preview"] }));
  const retried = container.querySelector("img")!;
  expect(retried).not.toBe(original);
  expect(retried.getAttribute("src")).toBe(original.getAttribute("src"));
  expect(screen.getByText("Preparing preview")).toBeTruthy();
  fireEvent.load(retried);
  expect(screen.queryByRole("status")).toBeNull();
});

it.each([false, true])("keeps unsupported-file guidance and its command localized (desktop=%s)", async (desktop) => {
  vi.mocked(isDesktopRuntime).mockReturnValue(desktop);
  const { rerender } = render(localized(<BinaryFilePlaceholder {...file} />, "zh"));
  expect(screen.getByText(MESSAGES.zh["workspace_file.unsupported_preview_title"])).toBeTruthy();
  const descriptionKey = desktop ? "workspace_file.unsupported_preview_reveal" : "workspace_file.unsupported_preview_download";
  expect(screen.getByText(MESSAGES.zh[descriptionKey])).toBeTruthy();
  rerender(localized(<BinaryFilePlaceholder {...file} />));
  expect(screen.queryByText(MESSAGES.zh[descriptionKey])).toBeNull();
  expect(screen.getByText(MESSAGES.en[descriptionKey])).toBeTruthy();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  const actionKey = desktop ? "workspace_file.reveal_named" : "workspace_file.download_named";
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en[actionKey].replace("{name}", file.fileName) }));
  expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith(file.agentId, file.path, file.fileName);
});

it("preserves the DOCX measurement host while loading and keeps failure retry independent of chrome", async () => {
  const containerRef = createRef<HTMLDivElement>();
  const styleContainerRef = createRef<HTMLDivElement>();
  const viewportRef = createRef<HTMLDivElement>();
  const retryPreview = vi.fn();
  const props = { ...file, containerRef, styleContainerRef, viewportRef, previewScale: 0.75, retryPreview };
  const { rerender } = render(localized(<DocumentPreviewView {...props} status={{ state: "loading" }} />));
  const host = containerRef.current;
  const styles = styleContainerRef.current;
  expect(host).not.toBeNull();
  expect(host?.style.getPropertyValue("--docx-preview-scale")).toBe("0.75");
  expect(screen.getAllByRole("status")).toHaveLength(1);
  rerender(localized(<DocumentPreviewView {...props} status={{ state: "loaded" }} />));
  expect(containerRef.current).toBe(host);
  expect(styleContainerRef.current).toBe(styles);
  expect(screen.queryByRole("status")).toBeNull();
  rerender(localized(<DocumentPreviewView {...props} status={{ state: "error" }} />));
  expect(containerRef.current).toBe(host);
  expect(host?.hasAttribute("inert")).toBe(true);
  expect(host?.getAttribute("aria-hidden")).toBe("true");
  expect(styleContainerRef.current).toBe(styles);
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["workspace_file.retry_preview"] }));
  expect(retryPreview).toHaveBeenCalledTimes(1);
  expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
});

it("updates text loading copy with the locale and removes it when content is ready", () => {
  const props = { agentId: file.agentId, content: "Saved contents", fileName: "report.txt", fileType: "text" as const, isStreaming: false };
  const { rerender } = render(localized(<TextFileContent {...props} isLoading />, "zh"));
  expect(screen.getByText(MESSAGES.zh["workspace_file.preview_loading"])).toBeTruthy();
  rerender(localized(<TextFileContent {...props} isLoading />));
  expect(screen.getByText("Preparing preview")).toBeTruthy();
  rerender(localized(<TextFileContent {...props} isLoading={false} />));
  expect(screen.queryByRole("status")).toBeNull();
  expect(screen.getByText("Saved contents")).toBeTruthy();
});
