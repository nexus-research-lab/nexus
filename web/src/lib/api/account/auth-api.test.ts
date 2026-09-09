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

  it("keeps the Desktop local owner while projecting the remote account", async () => {
    requestApi
      .mockResolvedValueOnce({
        auth_required: false,
        authenticated: true,
        auth_method: "local",
        password_login_enabled: false,
        user_id: "__system__",
        username: "local",
      })
      .mockResolvedValueOnce({
        auth_required: true,
        authenticated: true,
        auth_method: "password",
        password_login_enabled: true,
        user_id: "control-user",
        username: "lee",
      });

    await expect(getAuthStatus()).resolves.toMatchObject({
      auth_required: false,
      authenticated: true,
      auth_method: "password",
      user_id: "__system__",
      username: "lee",
    });
  });
});
