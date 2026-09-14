// INPUT: Controlled Range results, file/owner changes and explicit navigation.
// OUTPUT: One current chunk, exact offsets, safe reset/retry and named keyboard scrolling.
// POS: Offline controller/view regressions; API results never perform real downloads.
import type { ReactNode } from "react";
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { getWorkspaceFileTextChunkApi } from "@/lib/api/agent/agent-api";
import { advanceAuthOwnerScopeGeneration, publishAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { WorkspaceFileTextChunk } from "@/types/agent/agent";
import { LargeTextFilePreview } from "./large-text-file-preview";
import { useLargeTextFilePreview } from "./use-large-text-file-preview";

vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: () => false }));
vi.mock("@/lib/api/agent/agent-api", () => ({ getWorkspaceFileTextChunkApi: vi.fn(), downloadWorkspaceFileApi: vi.fn() }));

interface Read {
  agent: string;
  path: string;
  offset: number;
  signal: AbortSignal;
  resolve: (chunk: WorkspaceFileTextChunk) => void;
  reject: (error: Error) => void;
}
let reads: Read[];
beforeEach(() => {
  reads = [];
  vi.mocked(getWorkspaceFileTextChunkApi).mockImplementation((agent, path, offset, signal) => new Promise((resolve, reject) => {
    reads.push({ agent, path, offset, signal, resolve, reject });
  }));
});
function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}
const file = { agentId: "agent-a", path: "one/log.txt", fileName: "log.txt", isPreviewFocused: false, onTogglePreviewFocus: vi.fn() };
const first = { content: "你好", offset: 0, nextOffset: 6, size: 12 };
const last = { content: "世界", offset: 6, nextOffset: null, size: 12 };
async function resolve(index: number, chunk: WorkspaceFileTextChunk = first) { await act(async () => { reads[index].resolve(chunk); }); }
async function reject(index: number) { await act(async () => { reads[index].reject(new Error("internal read diagnostic")); }); }
const button = (key: "previous_chunk" | "next_chunk" | "load_from_start") => screen.getByRole("button", { name: MESSAGES.en[`workspace_file.${key}`] });

it("reads exact returned offsets, never joins chunks and uses fresh offsets after revisiting", async () => {
  render(localized(<LargeTextFilePreview {...file} />));
  expect(reads[0]).toMatchObject({ agent: file.agentId, path: file.path, offset: 0 });
  expect(screen.getAllByRole("status")).toHaveLength(1);
  await resolve(0);
  expect((button("previous_chunk") as HTMLButtonElement).disabled).toBe(true);
  const original = screen.getByRole("region", { name: file.fileName });
  expect(original.textContent).toBe(first.content);
  original.focus();
  expect(document.activeElement).toBe(original);
  await userEvent.tab({ shift: true });
  await userEvent.tab();
  expect(document.activeElement).toBe(original);
  const nextButton = button("next_chunk");
  await userEvent.click(nextButton);
  expect(button("next_chunk")).toBe(nextButton);
  expect((nextButton as HTMLButtonElement).disabled).toBe(true);
  expect(screen.queryByRole("region")).toBeNull();
  expect(reads[1].offset).toBe(6);
  await resolve(1, last);
  expect(button("next_chunk")).toBe(nextButton);
  expect(screen.getByRole("region").textContent).toBe(last.content);
  expect(screen.getByRole("region")).not.toBe(original);
  expect((button("next_chunk") as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(button("previous_chunk"));
  expect(reads[2].offset).toBe(0);
  await resolve(2, { ...first, content: "新内容", nextOffset: 9 });
  fireEvent.click(button("next_chunk"));
  expect(reads[3].offset).toBe(9);
  await resolve(3, { content: "尾", offset: 9, nextOffset: null, size: 12 });
  expect(screen.getByRole("region").textContent).toBe("尾");
  expect(reads).toHaveLength(4);
});

it("uses one localized error and explicitly restarts from byte zero after a later page fails", async () => {
  const view = render(localized(<LargeTextFilePreview {...file} />));
  await resolve(0);
  fireEvent.click(button("next_chunk")); await reject(1);
  expect(screen.queryByRole("region")).toBeNull();
  expect(screen.getAllByRole("status")).toHaveLength(1);
  expect(screen.queryByText("internal read diagnostic")).toBeNull();
  view.rerender(localized(<LargeTextFilePreview {...file} />, "zh"));
  expect(screen.getByText(MESSAGES.zh["workspace_file.chunk_load_failed"])).toBeTruthy();
  expect(reads).toHaveLength(2);
  fireEvent.click(screen.getByRole("button", { name: MESSAGES.zh["workspace_file.load_from_start"] }));
  expect(reads[2].offset).toBe(0);
  await resolve(2);
  expect((screen.getByRole("button", { name: MESSAGES.zh["workspace_file.previous_chunk"] }) as HTMLButtonElement).disabled).toBe(true);
});

it.each([{ path: "two/log.txt" }, { agentId: "agent-b" }])("resets page state and rejects an old read after scope change: %j", async (change) => {
  const view = render(localized(<LargeTextFilePreview {...file} />));
  await resolve(0); fireEvent.click(button("next_chunk"));
  view.rerender(localized(<LargeTextFilePreview {...file} {...change} />));
  expect(reads[1].signal.aborted).toBe(true);
  expect(reads[2]).toMatchObject({ agent: change.agentId ?? file.agentId, path: change.path ?? file.path, offset: 0 });
  await resolve(1, last);
  expect(screen.queryByRole("region")).toBeNull();
  await resolve(2, { ...first, content: "current file" });
  expect(screen.getByRole("region").textContent).toBe("current file");
});

it.each(["resolve", "reject"] as const)("fences owner changes before publication when the old read will %s", async (outcome) => {
  render(localized(<LargeTextFilePreview {...file} />));
  advanceAuthOwnerScopeGeneration();
  if (outcome === "resolve") await resolve(0); else await reject(0);
  expect(screen.queryByRole("region")).toBeNull();
  expect(screen.queryByText(MESSAGES.en["workspace_file.chunk_load_failed"])).toBeNull();
  act(() => publishAuthOwnerScopeGeneration());
  expect(reads[0].signal.aborted).toBe(true);
  expect(reads[1].offset).toBe(0);
  await resolve(1, { ...first, content: "current owner" });
  expect(screen.getByRole("region").textContent).toBe("current owner");
  act(() => { advanceAuthOwnerScopeGeneration(); publishAuthOwnerScopeGeneration(); });
  expect(screen.queryByRole("region")).toBeNull();
});

it("does not skip a page on repeated navigation before the next render", async () => {
  const { result } = renderHook(() => useLargeTextFilePreview(file.agentId, file.path));
  await resolve(0);
  act(() => { result.current.loadNext(); result.current.loadNext(); });
  expect(reads).toHaveLength(2);
  expect(reads[1].offset).toBe(6);
  expect(result.current.pageIndex).toBe(1);
  await resolve(1, last);
  act(() => result.current.loadNext());
  expect(reads).toHaveLength(2);
});

it("aborts on unmount and ignores a transport that still rejects afterward", async () => {
  const view = render(localized(<LargeTextFilePreview {...file} />));
  view.unmount();
  expect(reads[0].signal.aborted).toBe(true);
  await reject(0);
  expect(reads).toHaveLength(1);
});
