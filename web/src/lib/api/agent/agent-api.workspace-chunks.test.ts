// INPUT: Controlled HTTP Range responses and UTF-8 bytes crossing the chunk limit.
// OUTPUT: Bounded requests preserve exact file/offset and never split or replace characters.
// POS: Offline transport regressions; the existing API is exercised without network I/O.
// @vitest-environment node
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { getWorkspaceFileTextChunkApi } from "./agent-api";

vi.mock("@/config/desktop-runtime", () => ({ applyDesktopRequestHeaders: vi.fn(), getDesktopRuntimeConfig: () => null, isDesktopRuntime: () => false }));
const fetchMock = vi.fn<typeof fetch>();
beforeEach(() => { fetchMock.mockReset(); vi.stubGlobal("fetch", fetchMock); });
afterEach(() => vi.unstubAllGlobals());
const limit = 512 * 1024;
function response(bytes: Uint8Array, offset: number, size: number) {
  return new Response(bytes as BodyInit, { status: 206, headers: {
    "Content-Range": `bytes ${offset}-${offset + bytes.length - 1}/${size}`,
    "Content-Length": String(bytes.length),
  } });
}

it("stops before a multibyte boundary and resumes with the complete character", async () => {
  const prefix = "a".repeat(limit - 2);
  const bytes = new TextEncoder().encode(`${prefix}😀尾`);
  fetchMock.mockResolvedValueOnce(response(bytes.slice(0, limit + 3), 0, bytes.length));
  const controller = new AbortController();
  const first = await getWorkspaceFileTextChunkApi("file-agent", "output/日志.txt", 0, controller.signal);
  expect(first).toEqual({ content: prefix, offset: 0, nextOffset: limit - 2, size: bytes.length });
  const [url, options] = fetchMock.mock.calls[0];
  expect(String(url)).toContain("/agents/file-agent/workspace/download?");
  expect(new URL(String(url), "http://fixture.invalid").searchParams.get("path")).toBe("output/日志.txt");
  expect(options).toMatchObject({ method: "GET", credentials: "include", signal: controller.signal });
  expect((options?.headers as Headers).get("Range")).toBe(`bytes=0-${limit + 2}`);
  fetchMock.mockResolvedValueOnce(response(bytes.slice(limit - 2), limit - 2, bytes.length));
  const next = await getWorkspaceFileTextChunkApi("file-agent", "output/日志.txt", first.nextOffset!, controller.signal);
  expect(next).toEqual({ content: "😀尾", offset: limit - 2, nextOffset: null, size: bytes.length });
  expect((fetchMock.mock.calls[1][1]?.headers as Headers).get("Range")).toBe(`bytes=${limit - 2}-${2 * limit}`);
});

it.each([
  { status: 200, range: "bytes 0-3/4", length: "4" },
  { status: 206, range: "bytes 1-4/5", length: "4" },
  { status: 206, range: `bytes 0-${limit + 3}/${limit + 4}`, length: String(limit + 4) },
])("cancels an unbounded or mismatched response before reading its body: %j", async ({ status, range, length }) => {
  const invalid = new Response("body", { status, headers: { "Content-Range": range, "Content-Length": length } });
  const read = vi.spyOn(invalid, "arrayBuffer");
  const cancel = vi.spyOn(invalid.body!, "cancel");
  fetchMock.mockResolvedValueOnce(invalid);
  await expect(getWorkspaceFileTextChunkApi("agent", "file.txt", 0, new AbortController().signal)).rejects.toThrow();
  expect(read).not.toHaveBeenCalled();
  expect(cancel).toHaveBeenCalledTimes(1);
});

it("rejects malformed UTF-8 instead of returning replacement characters", async () => {
  const malformed = new Uint8Array([0xff, 0xff, 0xff, 0xff]);
  fetchMock.mockResolvedValueOnce(response(malformed, 0, malformed.length));
  await expect(getWorkspaceFileTextChunkApi("agent", "file.txt", 0, new AbortController().signal)).rejects.toThrow();
});
