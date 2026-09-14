// INPUT: Device authorization sessions and clipboard completion.
// OUTPUT: Copy feedback belongs to its session and releases its timer on unmount.
// POS: Device Flow dialog regression; polling and clipboard transport are isolated.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { isDesktopBridgeAvailable, openDesktopExternalURL } from "@/lib/desktop-bridge";
import { writeTextToClipboard } from "@/shared/lib/browser/clipboard";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES, LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ConnectorDeviceAuthDialog } from "./connector-device-auth-dialog";
vi.mock("@/lib/desktop-bridge", () => ({ isDesktopBridgeAvailable: vi.fn().mockReturnValue(false), openDesktopExternalURL: vi.fn() }));
vi.mock("./use-connector-device-auth", () => ({ useConnectorDeviceAuth: vi.fn() }));
vi.mock("@/shared/lib/browser/clipboard", () => ({ writeTextToClipboard: vi.fn().mockResolvedValue(true) }));
beforeEach(() => { localStorage.setItem(LOCALE_STORAGE_KEY, "zh"); });
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); vi.clearAllMocks(); });
it("resets copied feedback between sessions and clears its timer on unmount", async () => {
  vi.useFakeTimers();
  const timer = vi.spyOn(globalThis, "setTimeout");
  const clear = vi.spyOn(globalThis, "clearTimeout");
  const view = (device: string) => <I18nProvider><ConnectorDeviceAuthDialog
    session={{ connector_id: "github", device_code: device, user_code: device, verification_uri: "https://github.com/login/device", expires_in: 600, interval: 5 }}
    onCancel={vi.fn()} onClose={vi.fn()} onConnected={vi.fn()} onError={vi.fn()} onNext={vi.fn()} onOpenWebAuthUrl={vi.fn()}
  /></I18nProvider>;
  const { rerender, unmount } = render(view("FIRST"));
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "复制授权码" })));
  expect(writeTextToClipboard).toHaveBeenLastCalledWith("FIRST");
  expect(screen.getByRole("button", { name: "已复制授权码" })).toBeTruthy();
  rerender(view("SECOND"));
  expect(screen.queryByRole("button", { name: "已复制授权码" })).toBeNull();
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "复制授权码" })));
  expect(writeTextToClipboard).toHaveBeenLastCalledWith("SECOND");
  const copyTimerIndex = timer.mock.calls.findLastIndex((args) => args[1] === 1400);
  expect(copyTimerIndex).toBeGreaterThanOrEqual(0);
  const copyTimer = timer.mock.results[copyTimerIndex].value;
  unmount();
  expect(clear).toHaveBeenCalledWith(copyTimer);
});


it("ignores a clipboard failure from the previous authorization session", async () => {
  const error = vi.fn();
  const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
  let finish!: (result: boolean) => void;
  vi.mocked(writeTextToClipboard).mockImplementationOnce(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const view = (device: string) => <I18nProvider><ConnectorDeviceAuthDialog
    session={{ connector_id: "github", device_code: device, user_code: device, verification_uri: "https://github.com/login/device", expires_in: 600, interval: 5 }}
    onCancel={vi.fn()} onClose={vi.fn()} onConnected={vi.fn()} onError={error} onNext={vi.fn()} onOpenWebAuthUrl={vi.fn()}
  /></I18nProvider>;
  const { rerender } = render(view("OLD"));
  fireEvent.click(screen.getByRole("button", { name: "复制授权码" }));
  rerender(view("CURRENT"));
  await act(async () => finish(false));
  expect(error).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "复制授权码" })).toBeTruthy();
  vi.mocked(writeTextToClipboard).mockResolvedValueOnce(false);
  await act(async () => fireEvent.click(screen.getByRole("button", { name: "复制授权码" })));
  expect(error).toHaveBeenCalledExactlyOnceWith("复制授权码失败");
  expect(log).toHaveBeenCalledTimes(2);
});


it("does not overwrite the current authorization hint with a previous desktop open result", async () => {
  vi.mocked(isDesktopBridgeAvailable).mockReturnValue(true);
  let finish!: () => void;
  vi.mocked(openDesktopExternalURL)
    .mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve; }))
    .mockRejectedValueOnce(new Error("blocked"));
  const openWeb = vi.fn();
  const view = (device: string) => <I18nProvider><ConnectorDeviceAuthDialog
    session={{ connector_id: "feishu-docx", stage: "user_authorization", device_code: device, user_code: device, verification_uri: `https://example.com/${device}`, expires_in: 600, interval: 5 }}
    onCancel={vi.fn()} onClose={vi.fn()} onConnected={vi.fn()} onError={vi.fn()} onNext={vi.fn()} onOpenWebAuthUrl={openWeb}
  /></I18nProvider>;
  const { rerender } = render(view("OLD"));
  await act(async () => rerender(view("NEW")));
  expect(screen.getByText("授权页未自动打开，请手动继续")).toBeTruthy();
  await act(async () => finish());
  expect(screen.getByText("授权页未自动打开，请手动继续")).toBeTruthy();
  expect(screen.queryByText("已打开飞书授权页，等待确认")).toBeNull();
  expect(openDesktopExternalURL).toHaveBeenCalledTimes(2);
  expect(openWeb).not.toHaveBeenCalled();
  vi.mocked(isDesktopBridgeAvailable).mockReturnValue(false);
});

it("updates the visible status and title on locale changes without opening authorization again", () => {
  const openWeb = vi.fn().mockReturnValue(false);
  const session = { connector_id: "feishu-docx", stage: "user_authorization" as const, device_code: "stable", user_code: "CODE", verification_uri: "https://example.com/auth", expires_in: 600, interval: 5 };
  const props = { session, onCancel: vi.fn(), onClose: vi.fn(), onConnected: vi.fn(), onError: vi.fn(), onNext: vi.fn(), onOpenWebAuthUrl: openWeb };
  const view = (locale: "zh" | "en") => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><ConnectorDeviceAuthDialog {...props} /></I18N_CONTEXT.Provider>;
  const { rerender } = render(view("zh"));
  expect(screen.getByRole("dialog", { name: "连接飞书云文档" })).toBeTruthy();
  rerender(view("en"));
  expect(screen.getByRole("dialog", { name: "Connect Feishu Docs" })).toBeTruthy();
  expect(screen.getByText("Authorization did not open automatically. Continue manually.")).toBeTruthy();
  expect(openWeb).toHaveBeenCalledOnce();
});
