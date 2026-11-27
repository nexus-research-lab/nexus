// INPUT: 本地待确认队列与 Room 订阅快照的后端 pending 全集。
// OUTPUT: 丢失 resolved 事件的残留请求被移除，自动化确认与仍待响应请求原样保留。
// POS: pending-permission-model 权威快照对账回归。
import { describe, expect, it } from "vitest";

import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { reconcilePendingPermissionsWithAuthoritativeIds } from "./pending-permission-model";

function permission(
  requestId: string,
  overrides: Partial<PendingPermission> = {},
): PendingPermission {
  return {
    request_id: requestId,
    tool_name: "AskUserQuestion",
    tool_input: {},
    interaction_mode: "question",
    ...overrides,
  };
}

describe("reconcilePendingPermissionsWithAuthoritativeIds", () => {

  it("clears every non-automation request when the snapshot is empty", () => {
    const automation = permission("perm_automation", { source: "automation" });

    const next = reconcilePendingPermissionsWithAuthoritativeIds(
      [permission("perm_stale"), automation],
      [],
    );

    expect(next).toEqual([automation]);
  });

  it("returns the same array when nothing changes", () => {
    const current = [permission("perm_live")];

    expect(
      reconcilePendingPermissionsWithAuthoritativeIds(current, [" perm_live ", 7]),
    ).toBe(current);
  });
});
