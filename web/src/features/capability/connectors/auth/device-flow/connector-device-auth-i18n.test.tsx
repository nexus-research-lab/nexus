// INPUT: Device auth locale changes while a provider poll is in flight.
// OUTPUT: Current-language errors without restarting a poll or exposing provider text.
// POS: Device Flow language and lifecycle regression.
import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { pollConnectorDeviceAuthApi } from "@/lib/api/capability/connector-api";
import { useConnectorDeviceAuth } from "./use-connector-device-auth";
vi.mock("@/lib/api/capability/connector-api", () => ({ pollConnectorDeviceAuthApi: vi.fn() }));
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks(); });
it("translates an in-flight rejection in the current locale without another poll", async () => {
  vi.useFakeTimers();
  let finish!: (value: { status: "denied"; message: string }) => void;
  vi.mocked(pollConnectorDeviceAuthApi).mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  let locale: Locale = "zh";
  const wrapper = ({ children }: { children: ReactNode }) => {
    const value: I18nContextValue = { locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] };
    return <I18N_CONTEXT.Provider value={value}>{children}</I18N_CONTEXT.Provider>;
  };
  const session = { connector_id: "github", device_code: "same", user_code: "SAME", verification_uri: "https://github.com/login/device", expires_in: 600, interval: 1 };
  const callbacks = { onClose: vi.fn(), onConnected: vi.fn(), onError: vi.fn(), onMessage: vi.fn(), onNext: vi.fn() };
  const view = renderHook(() => useConnectorDeviceAuth({ ...callbacks, session }), { wrapper });
  await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
  locale = "en";
  view.rerender();
  await act(async () => { finish({ status: "denied", message: "服务器秘密" }); });
  expect(pollConnectorDeviceAuthApi).toHaveBeenCalledOnce();
  expect(callbacks.onError).toHaveBeenCalledExactlyOnceWith("Authorization not completed", "not_connected");
  expect(callbacks.onClose).toHaveBeenCalledOnce();
});
