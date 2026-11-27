// INPUT: Complete/partial HTML and timed streaming updates.
// OUTPUT: One sandboxed frame or named source fallback, with bounded commits and timer cleanup.
// POS: Offline DOM/state tests; frame scripts, storage APIs and host rendering are not executed.
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HtmlFilePreview } from "./html-file-preview";

beforeEach(() => { vi.useFakeTimers(); vi.setSystemTime(1_000); });
afterEach(() => { cleanup(); vi.useRealTimers(); });
const complete = (text: string) => `<html><head><style>body{color:black}</style></head><body>${text}</body></html>`;
const frame = () => screen.getByTitle("report.html") as HTMLIFrameElement;
const advance = (milliseconds: number) => act(() => vi.advanceTimersByTime(milliseconds));

it.each([
  '<html><head data-preview="yes"><script>window.previewStep = "content";</script></head><body>Report</body></html>',
  '<html lang="en"><body><script>window.previewStep = "content";</script>Report</body></html>',
  '<section>Report<script>window.previewStep = "content";</script></section>',
])("preserves the storage shim before document scripts and the existing opaque sandbox", (content) => {
  render(<HtmlFilePreview content={content} title="report.html" />);
  const preview = frame();
  expect(preview.getAttribute("sandbox")).toBe("allow-downloads allow-forms allow-modals allow-popups allow-scripts");
  expect(preview.srcdoc).toContain('installStorage("localStorage")');
  expect(preview.srcdoc).toContain('installStorage("sessionStorage")');
  expect(preview.srcdoc.indexOf("createStorage")).toBeLessThan(preview.srcdoc.indexOf("window.previewStep"));
  expect(preview.srcdoc.match(/__nexus_preview_storage_test__/g)).toHaveLength(1);
  expect(preview.srcdoc).toContain("Report");
  expect(screen.queryByRole("region")).toBeNull();
});

it("keeps an incomplete streaming head as literal keyboard-readable source until ready", async () => {
  vi.useRealTimers();
  const content = "<html><head><style>body { color:";
  const view = render(<HtmlFilePreview content={content} isStreaming title="report.html" />);
  const source = screen.getByRole("region", { name: "report.html" });
  expect(source.textContent).toBe(content);
  expect(screen.queryByTitle("report.html")).toBeNull();
  await userEvent.tab();
  expect(document.activeElement).toBe(source);
  view.rerender(<HtmlFilePreview content={complete("ready")} isStreaming title="report.html" />);
  expect(screen.queryByRole("region")).toBeNull();
  expect(frame().srcdoc).toContain("ready");
});

it("preserves the committed frame while a new head is partial and cleans up on unmount", () => {
  const view = render(<HtmlFilePreview content={complete("first")} isStreaming title="report.html" />);
  advance(30);
  view.rerender(<HtmlFilePreview content={complete("intermediate")} isStreaming title="report.html" />);
  view.rerender(<HtmlFilePreview content="<head><style>body {" isStreaming title="report.html" />);
  advance(500);
  expect(frame().srcdoc).toContain("first");
  expect(vi.getTimerCount()).toBe(0);
  view.rerender(<HtmlFilePreview content={complete("next")} isStreaming title="report.html" />);
  advance(10);
  view.rerender(<HtmlFilePreview content={complete("pending")} isStreaming title="report.html" />);
  expect(vi.getTimerCount()).toBe(1);
  view.unmount();
  expect(vi.getTimerCount()).toBe(0);
});
