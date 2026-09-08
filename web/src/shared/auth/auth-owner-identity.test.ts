// INPUT: 已认证、未认证、本地主体与边界长度的身份事实。
// OUTPUT: 证明共享投影保留既有 owner 优先级、长度边界、本地回退与 marker 格式。
// POS: 无副作用身份合同测试；清理顺序、存储失败和旧回调栅栏由 App 集成测试负责。

import { describe, expect, it } from "vitest";

import {
  isValidPersistedAuthOwnerScope,
  resolveAuthOwnerScope,
} from "./auth-owner-identity";

describe("auth owner identity contract", () => {
  const authenticated = { authenticated: true, auth_required: true, username: "alice" };

  it("prefers the durable owner ID and only falls back to a valid username", () => {
    expect(resolveAuthOwnerScope({ ...authenticated, user_id: "  owner-a  " }))
      .toBe("user-id:owner-a");
    expect(resolveAuthOwnerScope({ ...authenticated, user_id: " ", username: " alice " }))
      .toBe("username:alice");
    expect(resolveAuthOwnerScope({ ...authenticated, user_id: "x".repeat(512) }))
      .toBe(`user-id:${"x".repeat(512)}`);
    expect(resolveAuthOwnerScope({ ...authenticated, user_id: "x".repeat(513) }))
      .toBe("username:alice");
    expect(resolveAuthOwnerScope({ ...authenticated, username: "x".repeat(513) }))
      .toBeNull();
  });

  it("keeps unauthenticated facts unbound and reserves the local fallback for authenticated local mode", () => {
    expect(resolveAuthOwnerScope({ ...authenticated, authenticated: false, user_id: "owner-a" }))
      .toBeNull();
    expect(resolveAuthOwnerScope({ ...authenticated, username: null }))
      .toBeNull();
    expect(resolveAuthOwnerScope({ ...authenticated, auth_required: false, username: null }))
      .toBe("local-system");
    expect(resolveAuthOwnerScope({ ...authenticated, authenticated: false, auth_required: false, username: null }))
      .toBeNull();
  });

  it("recognizes the persisted namespace without rewriting existing markers", () => {
    for (const marker of ["local-system", "user-id:owner-a", "username:alice"]) {
      expect(isValidPersistedAuthOwnerScope(marker), marker).toBe(true);
    }
    for (const marker of ["", "owner-a", " local-system ", "user:alice", "local-system:owner-a"]) {
      expect(isValidPersistedAuthOwnerScope(marker), marker).toBe(false);
    }
  });
});
