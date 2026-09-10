// INPUT: RichMail 配对弹窗、受控轮询响应与时间推进。
// OUTPUT: 用户状态不回显服务端文本，终态保留精确成功/失败语义并停止轮询。
// POS: 配对视图/Hook 集成回归；不连接本机服务。
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { pollConnectorLocalPairingApi } from "@/lib/api/capability/connector-api";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import { RichMailPairingDialog } from "./richmail-pairing-dialog";

vi.mock("@/lib/api/capability/connector-api", () => ({pollConnectorLocalPairingApi: vi.fn()}));
beforeEach(() => { vi.useFakeTimers(); vi.resetAllMocks(); localStorage.setItem(LOCALE_STORAGE_KEY, "en"); });
afterEach(() => vi.useRealTimers());
const session = {connector_id: "richmail", attempt_token: "opaque-test-attempt", endpoint: "http://127.0.0.1:3100", expires_in: 60, interval: 1};
function setup() {
  const callbacks = {onClose: vi.fn(), onCancel: vi.fn(), onConnected: vi.fn().mockResolvedValue(undefined), onError: vi.fn()};
  const view = render(<I18nProvider><RichMailPairingDialog session={session} {...callbacks} /></I18nProvider>);
  return {...callbacks, ...view};
}

it("renders product guidance and fixed status while preserving successful pairing", async () => {
  vi.mocked(pollConnectorLocalPairingApi)
    .mockResolvedValueOnce({status: "pending", message: "private diagnostic"})
    .mockResolvedValueOnce({status: "connected", message: "secret response"});
  const view = setup();
  expect(screen.getByRole("dialog", {name: "Connect RichMail"})).toBeTruthy();
  expect(screen.getByText("Approve the connection in the RichMail dialog")).toBeTruthy();
  expect(screen.queryByText(/Token/)).toBeNull();
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(screen.getByText("Waiting for approval in RichMail")).toBeTruthy();
  expect(screen.queryByText("private diagnostic")).toBeNull();
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(view.onConnected).toHaveBeenCalledExactlyOnceWith("richmail");
  expect(view.onClose).toHaveBeenCalledOnce();
  expect(view.onError).not.toHaveBeenCalled();
  expect(screen.queryByText("secret response")).toBeNull();
  expect(pollConnectorLocalPairingApi).toHaveBeenCalledWith("richmail", "opaque-test-attempt");
  await act(async () => vi.advanceTimersByTimeAsync(5000));
  expect(pollConnectorLocalPairingApi).toHaveBeenCalledTimes(2);
  view.unmount();
});

it.each([
  ["expired", "The RichMail pairing request expired"],
  ["denied", "RichMail did not approve this connection"],
] as const)("projects %s without showing backend prose", async (status, message) => {
  vi.mocked(pollConnectorLocalPairingApi).mockResolvedValue({status, message: "private diagnostic"});
  const view = setup();
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(view.onError).toHaveBeenCalledExactlyOnceWith(message, "not_connected");
  expect(view.onClose).toHaveBeenCalledOnce();
  expect(view.onConnected).not.toHaveBeenCalled();
  view.unmount();
});

it("keeps transport failure unknown instead of reporting denial", async () => {
  vi.mocked(pollConnectorLocalPairingApi).mockRejectedValue(new Error("private transport diagnostic"));
  const view = setup();
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(view.onError).toHaveBeenCalledExactlyOnceWith("The RichMail pairing result could not be confirmed", "outcome_unknown");
  expect(view.onClose).toHaveBeenCalledOnce();
  expect(view.onConnected).not.toHaveBeenCalled();
  view.unmount();
});
