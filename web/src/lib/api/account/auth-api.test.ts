import { beforeEach, describe, expect, it, vi } from "vitest";

const { requestApi } = vi.hoisted(() => ({ requestApi: vi.fn() }));

vi.mock("@/config/desktop-runtime", () => ({
  getDesktopRuntimeConfig: () => null,
  isDesktopRuntime: () => true,
}));
vi.mock("@/lib/api/core/http", () => ({ requestApi }));

import { getAuthStatus } from "./auth-api";

describe("getAuthStatus", () => {
  beforeEach(() => requestApi.mockReset());

  it.each([false, undefined])("keeps unavailable multiplayer disabled independently of the remote organization (%s)", async (enabled) => {
    requestApi.mockResolvedValueOnce({ authenticated: true, user_id: "local", auth_method: "local" });
    if (enabled === undefined) requestApi.mockRejectedValueOnce(new Error("not deployed"));
    else requestApi.mockResolvedValueOnce({ enabled });
    requestApi.mockResolvedValueOnce({ authenticated: true, user_id: "remote", organization_id: "org", auth_method: "password" });
    await expect(getAuthStatus()).resolves.toMatchObject({
      user_id: "local",
      organization_id: "org",
      multiplayer_enabled: false,
      // 部署明确关闭不提示；探测失败（旧服务 404、网络）才标记不可用。
      multiplayer_unavailable: enabled === undefined,
    });
  });

  it("issues local, capability and Control reads together", async () => {
    let resolveLocal: (value: unknown) => void = () => {};
    requestApi
      .mockReturnValueOnce(new Promise((next) => {
        resolveLocal = next;
      }))
      .mockResolvedValueOnce({ enabled: true })
      .mockRejectedValueOnce(new Error("offline"));

    const status = getAuthStatus();
    expect(requestApi).toHaveBeenCalledTimes(3);
    resolveLocal({ authenticated: true, user_id: "local", password_login_enabled: false });
    await expect(status).resolves.toMatchObject({
      user_id: "local",
      multiplayer_enabled: true,
      password_login_enabled: true,
    });
  });
});
