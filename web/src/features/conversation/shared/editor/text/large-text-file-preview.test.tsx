// INPUT: Controlled Range results, file/owner changes and explicit navigation.
// OUTPUT: One current chunk, exact offsets, safe reset/retry and named keyboard scrolling.
// POS: Offline controller/view regressions; API results never perform real downloads.
import type { ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { getWorkspaceFileTextChunkApi } from "@/lib/api/agent/agent-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { WorkspaceFileTextChunk } from "@/types/agent/agent";
import { LargeTextFilePreview } from "./large-text-file-preview";

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
async function resolve(index: number, chunk: WorkspaceFileTextChunk = first) { await act(async () => { reads[index].resolve(chunk); }); }
async function reject(index: number) { await act(async () => { reads[index].reject(new Error("internal read diagnostic")); }); }
const button = (key: "previous_chunk" | "next_chunk" | "load_from_start") => screen.getByRole("button", { name: MESSAGES.en[`workspace_file.${key}`] });

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

it("aborts on unmount and ignores a transport that still rejects afterward", async () => {
  const view = render(localized(<LargeTextFilePreview {...file} />));
  view.unmount();
  expect(reads[0].signal.aborted).toBe(true);
  await reject(0);
  expect(reads).toHaveLength(1);
});
