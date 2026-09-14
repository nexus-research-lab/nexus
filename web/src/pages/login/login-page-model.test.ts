import { describe, expect, it } from "vitest";

import { buildLoginPageState } from "./login-page-model";

describe("buildLoginPageState", () => {
  it("keeps optional remote sign-in open for a Desktop local principal", () => {
    expect(buildLoginPageState({
      isBootstrapped: true,
      loading: false,
      redirectPath: "/launcher",
      status: {
        auth_required: false,
        authenticated: true,
        auth_method: "local",
        password_login_enabled: true,
        username: "local",
      },
    })).toEqual({ kind: "ready", formMode: "password" });
  });
});
