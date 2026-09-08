// INPUT: QR payload changes, delayed generation and native image errors.
// OUTPUT: Only current payload results are shown, with localized feedback and explicit payload visibility.
// POS: Offline QR lifecycle regressions; does not test scanning or authorization protocols.

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { QRCodeToDataURLOptions } from "qrcode";
import { beforeEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { UiQRCode } from "./qr-code";

const { toDataURL } = vi.hoisted(() => ({
  toDataURL: vi.fn<(value: string, options: QRCodeToDataURLOptions) => Promise<string>>(),
}));
vi.mock("qrcode", () => ({ toDataURL }));
beforeEach(() => { toDataURL.mockReset(); });

function qr(payload: string, locale: Locale = "en", showPayload = false) {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <UiQRCode alt="Scan code" payload={payload} showPayload={showPayload} />
  </I18N_CONTEXT.Provider>;
}
function deferred() {
  let resolve!: (value: string) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<string>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

it("reports a broken embedded image without exposing its payload and resets for a fresh image", () => {
  const first = "data:image/png;base64,first";
  const second = "data:image/png;base64,second";
  const { rerender, container } = render(qr(first));
  const oldImage = screen.getByRole("img", { name: "Scan code" });
  fireEvent.error(oldImage);
  expect(screen.queryByRole("img")).toBeNull();
  expect(screen.getByRole("status").textContent).toBe(MESSAGES.en["common.qr_failed"]);
  expect(container.textContent).not.toContain(first);
  rerender(qr(second));
  const freshImage = screen.getByRole("img", { name: "Scan code" });
  expect(freshImage.getAttribute("src")).toBe(second);
  fireEvent.error(oldImage);
  expect(screen.getByRole("img")).toBe(freshImage);
  expect(toDataURL).not.toHaveBeenCalled();
});

it.each(["resolve", "reject"] as const)("ignores a previous payload that later %s", async (outcome) => {
  const old = deferred();
  const fresh = deferred();
  vi.mocked(toDataURL).mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise);
  const { rerender } = render(qr("first-token"));
  await waitFor(() => expect(toDataURL).toHaveBeenCalledTimes(1));
  rerender(qr("second-token"));
  await waitFor(() => expect(toDataURL).toHaveBeenCalledTimes(2));
  await act(async () => fresh.resolve("data:image/png;base64,new"));
  await act(async () => outcome === "resolve" ? old.resolve("data:image/png;base64,old") : old.reject(new Error("old failure")));
  expect(screen.getByRole("img").getAttribute("src")).toBe("data:image/png;base64,new");
  expect(screen.queryByRole("status")).toBeNull();
});

it("localizes an in-flight generation without restarting it, then handles image decode failure", async () => {
  const generation = deferred();
  vi.mocked(toDataURL).mockReturnValue(generation.promise);
  const { rerender, container } = render(qr("private-token"));
  expect(screen.getByRole("status").textContent).toBe(MESSAGES.en["common.qr_loading"]);
  expect(screen.getByRole("status").getAttribute("aria-busy")).toBe("true");
  await waitFor(() => expect(toDataURL).toHaveBeenCalledTimes(1));
  rerender(qr("private-token", "zh"));
  expect(screen.getByRole("status").textContent).toBe(MESSAGES.zh["common.qr_loading"]);
  await act(async () => generation.resolve("data:image/png;base64,generated"));
  fireEvent.error(screen.getByRole("img"));
  expect(screen.getByRole("status").textContent).toBe(MESSAGES.zh["common.qr_failed"]);
  expect(toDataURL).toHaveBeenCalledTimes(1);
  expect(container.textContent).not.toContain("private-token");
});

it.each(["reject", "empty"] as const)("shows a generation %s fallback and reveals content only when requested", async (outcome) => {
  if (outcome === "reject") vi.mocked(toDataURL).mockRejectedValue(new Error("unavailable"));
  else vi.mocked(toDataURL).mockResolvedValue("");
  const { rerender, container } = render(qr("copyable-value", "en", true));
  await screen.findByText(MESSAGES.en["common.qr_failed_with_payload"]);
  expect(screen.getByText("copyable-value").tagName).toBe("CODE");
  rerender(qr("copyable-value"));
  expect(screen.getByText(MESSAGES.en["common.qr_failed"])).toBeTruthy();
  expect(container.textContent).not.toContain("copyable-value");
});

it("trims payloads without regenerating the same code for surrounding whitespace", async () => {
  vi.mocked(toDataURL).mockResolvedValue("data:image/png;base64,current");
  const { rerender } = render(qr("  current  "));
  const image = await screen.findByRole("img", { name: "Scan code" });
  expect(image.getAttribute("src")).toBe("data:image/png;base64,current");
  rerender(qr("current"));
  expect(screen.getByRole("img", { name: "Scan code" })).toBe(image);
  expect(toDataURL).toHaveBeenCalledTimes(1);
  expect(toDataURL).toHaveBeenCalledWith("current", expect.objectContaining({ width: 220, errorCorrectionLevel: "M" }));
});
