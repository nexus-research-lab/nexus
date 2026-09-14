// INPUT: Native load/error events, retries and exact file/owner changes.
// OUTPUT: Shared state transitions keep native frames/images scoped and file actions available.
// POS: Offline media regressions; native resources, PDF rendering and downloads are not loaded.
import type { ReactNode } from "react";
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { ImagePreview, PdfPreview } from "./media-file-preview";
import { useNativeMediaPreview } from "./use-native-media-preview";

vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: () => false }));
vi.mock("@/lib/api/agent/agent-api", () => ({
  getWorkspaceFilePreviewUrl: (agent: string, path: string) => `/preview/${agent}?path=${encodeURIComponent(path)}`,
  downloadWorkspaceFileApi: vi.fn(async () => undefined),
}));
function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}
const file = { agentId: "agent-a", path: "one/report", fileName: "Report", isPreviewFocused: false, onTogglePreviewFocus: vi.fn() };
const kinds = [{ Component: ImagePreview, tag: "img" }, { Component: PdfPreview, tag: "iframe" }] as const;

it("retries image failures with a fresh element and current localized feedback", async () => {
  const view = render(localized(<ImagePreview {...file} />));
  const original = view.container.querySelector("img")!;
  expect(screen.getAllByRole("status")).toHaveLength(1);
  fireEvent.error(original);
  expect(view.container.querySelector("img")).toBeNull();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.getByText(MESSAGES.en["workspace_file.image_preview_failed"])).toBeTruthy();
  view.rerender(localized(<ImagePreview {...file} />, "zh"));
  expect(screen.getByText(MESSAGES.zh["workspace_file.image_preview_failed"])).toBeTruthy();
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.zh["workspace_file.retry_preview"] }));
  const current = view.container.querySelector("img")!;
  expect(current).not.toBe(original);
  expect(current.getAttribute("src")).toBe(original.getAttribute("src"));
  fireEvent.load(original);
  expect(screen.getAllByRole("status")).toHaveLength(1);
  fireEvent.load(current);
  expect(screen.queryByRole("status")).toBeNull();
});

it("offers explicit PDF reload both while pending and after a frame load event", async () => {
  const view = render(localized(<PdfPreview {...file} />));
  const first = view.container.querySelector("iframe")!;
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.en["workspace_file.reload_preview"] }));
  const current = view.container.querySelector("iframe")!;
  expect(current).not.toBe(first);
  expect(current.getAttribute("src")).toBe(first.getAttribute("src"));
  expect(current.getAttribute("sandbox")).toBe("allow-downloads allow-same-origin");
  fireEvent.load(first);
  expect(screen.getAllByRole("status")).toHaveLength(1);
  fireEvent.load(current);
  expect(screen.queryByRole("status")).toBeNull();
  view.rerender(localized(<PdfPreview {...file} />, "zh"));
  await userEvent.click(screen.getByRole("button", { name: MESSAGES.zh["workspace_file.reload_preview"] }));
  expect(view.container.querySelector("iframe")).not.toBe(current);
  expect(screen.getAllByRole("status")).toHaveLength(1);
});

it.each(kinds)("preserves the $tag for chrome updates and reloads for file or owner changes", ({ Component, tag }) => {
  const view = render(localized(<Component {...file} />));
  const first = view.container.querySelector(tag)!;
  fireEvent.load(first);
  view.rerender(localized(<Component {...file} fileName="Renamed" isPreviewFocused />));
  expect(view.container.querySelector(tag)).toBe(first);
  expect(screen.queryByRole("status")).toBeNull();
  view.rerender(localized(<Component {...file} path="two/report" />));
  const second = view.container.querySelector(tag)!;
  expect(second).not.toBe(first);
  expect(second.getAttribute("src")).toBe("/preview/agent-a?path=two%2Freport");
  expect(screen.getAllByRole("status")).toHaveLength(1);
  fireEvent.error(first);
  fireEvent.load(second);
  expect(screen.queryByRole("status")).toBeNull();
  view.rerender(localized(<Component {...file} agentId="agent-b" />));
  const third = view.container.querySelector(tag)!;
  expect(third).not.toBe(second);
  fireEvent.load(third);
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  expect(view.container.querySelector(tag)).not.toBe(third);
  expect(screen.getAllByRole("status")).toHaveLength(1);
});

it("rejects retained callbacks from previous files, retries and unpublished owner generations", () => {
  const view = renderHook(({ path }) => useNativeMediaPreview("agent", path), { initialProps: { path: "first" } });
  const first = view.result.current;
  act(() => first.onLoad());
  view.rerender({ path: "second" });
  const second = view.result.current;
  act(() => { first.onError(); first.reloadPreview(); });
  expect(view.result.current.previewKey).toBe(second.previewKey);
  expect(view.result.current.loadState).toBe("loading");
  act(() => second.onError());
  expect(view.result.current.loadState).toBe("error");
  act(() => view.result.current.reloadPreview());
  expect(view.result.current.previewKey).not.toBe(second.previewKey);
  act(() => second.onLoad());
  expect(view.result.current.loadState).toBe("loading");
  view.rerender({ path: "first" });
  const returned = view.result.current;
  act(() => { first.onLoad(); first.reloadPreview(); });
  expect(view.result.current.loadState).toBe("loading");
  expect(view.result.current.previewKey).toBe(returned.previewKey);
  expect(returned.previewKey).not.toBe(first.previewKey);
  const priorOwner = view.result.current;
  advanceAuthOwnerScopeGeneration();
  act(() => { priorOwner.onError(); priorOwner.reloadPreview(); });
  expect(view.result.current.loadState).toBe("loading");
  expect(view.result.current.previewKey).toBe(priorOwner.previewKey);
  act(() => publishAuthOwnerScopeGeneration());
  expect(view.result.current.previewKey).not.toBe(priorOwner.previewKey);
  act(() => view.result.current.onLoad());
  expect(view.result.current.loadState).toBe("settled");
});
