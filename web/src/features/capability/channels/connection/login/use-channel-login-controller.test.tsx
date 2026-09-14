// INPUT: Exact login snapshots and verification/reconciliation responses.
// OUTPUT: Confirms direct successful responses refresh accounts without replaying login writes.
// POS: Channel login state-machine regression tests.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useChannelCommand } from "../use-channel-command";
import { useChannelLoginController } from "./use-channel-login-controller";

const api = vi.hoisted(() => ({ start: vi.fn(), verify: vi.fn(), read: vi.fn(), current: vi.fn() }));
vi.mock("@/lib/api/capability/channel-api", () => ({
  startChannelLoginApi: api.start,
  submitChannelLoginVerifyCodeApi: api.verify,
  getChannelLoginApi: api.read,
  getCurrentChannelLoginApi: api.current,
}));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
const snapshot = (status: string) => ({ login_id: "login-1", channel_type: "weixin-personal", status });
function setup(onCompleted = vi.fn().mockResolvedValue(undefined)) {
  const hook = renderHook(() => {
    const command = useChannelCommand();
    return useChannelLoginController({ channelType: "weixin-personal", enabled: true, onCompleted, ...command });
  });
  return { ...hook, onCompleted };
}
beforeEach(() => vi.resetAllMocks());

describe("channel login successful response reconciliation", () => {
  it("refreshes connected accounts when starting returns a completed login", async () => {
    api.start.mockResolvedValue(snapshot("succeeded"));
    const { result, onCompleted } = setup();
    await act(async () => { await result.current.startLogin(); });
    expect(onCompleted).toHaveBeenCalledTimes(1);
    expect(api.start).toHaveBeenCalledTimes(1);
  });

  it("refreshes a direct verification success and retries only the failed read", async () => {
    api.start.mockResolvedValue(snapshot("verify_code_required"));
    api.verify.mockResolvedValue(snapshot("succeeded"));
    const onCompleted = vi.fn().mockRejectedValueOnce(new Error("offline")).mockResolvedValue(undefined);
    const { result } = setup(onCompleted);
    await act(async () => { await result.current.startLogin(); });
    await act(async () => { expect(await result.current.submitVerifyCode("123456")).toBe(true); });
    expect(result.current.view?.status).toBe("succeeded");
    expect(result.current.recoveryNotice?.title).toBe("capability.channel_login_refresh_failed_title");
    await act(async () => { result.current.recoveryNotice?.action?.onClick(); });
    expect(onCompleted).toHaveBeenCalledTimes(2);
    expect(api.start).toHaveBeenCalledTimes(1);
    expect(api.verify).toHaveBeenCalledTimes(1);
    expect(result.current.recoveryNotice).toBeNull();
  });

  it("refreshes a verification success found by read-only reconciliation", async () => {
    api.start.mockResolvedValue(snapshot("verify_code_required"));
    api.verify.mockRejectedValue(new Error("disconnected"));
    api.read.mockResolvedValue(snapshot("succeeded"));
    const { result, onCompleted } = setup();
    await act(async () => { await result.current.startLogin(); });
    await act(async () => { await result.current.submitVerifyCode("123456"); });
    await act(async () => { result.current.recoveryNotice?.action?.onClick(); });
    expect(onCompleted).toHaveBeenCalledTimes(1);
    expect(api.verify).toHaveBeenCalledTimes(1);
    expect(api.read).toHaveBeenCalledWith("weixin-personal", "login-1");
  });
});
