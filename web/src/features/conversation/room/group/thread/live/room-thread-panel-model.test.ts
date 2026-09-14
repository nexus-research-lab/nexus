// INPUT: 精确 Thread 目标、实时切片和不完整名称目录。
// OUTPUT: 验证可读名称降级不改变权限过滤或命令归属。
// POS: Thread 名称与 exact scope 投影回归。

import { describe, expect, it, vi } from "vitest";
import { MESSAGES } from "@/shared/i18n/messages";
import { buildRoomThreadPanelModel } from "./room-thread-panel-model";
import type { RoomThreadLiveSource } from "./room-thread-live-store";

describe("Thread participant names", () => {
  it.each(["zh", "en"] as const)("keeps exact pending commands when a name is missing in %s", (locale) => {
    const onPermissionResponse = vi.fn(() => true);
    const permission = { request_id: "exact", agent_id: "private-agent-id", round_id: "round", agent_round_id: "agent-round", tool_name: "Write", tool_input: {} };
    const source: RoomThreadLiveSource = { agentAvatarMap: {}, agentNameMap: {}, messageGroups: new Map(), pendingSlotGroups: new Map(), roomAgentExecutionStateGroups: new Map(), onPermissionResponse,
      pendingPermissionGroups: new Map([["round", [permission, { ...permission, agent_id: "other", request_id: "other" }]]]) };
    const target = { agentId: "private-agent-id", roundId: "round", agentRoundId: "agent-round" };
    const t = (key: keyof typeof MESSAGES.zh) => MESSAGES[locale][key];
    const model = buildRoomThreadPanelModel(source, target, t)!;
    expect(model.agentName).toBe(MESSAGES[locale]["agent.name_fallback"]);
    expect(model.pendingPermissions).toEqual([permission]);
    expect(model.onPermissionResponse).toBe(onPermissionResponse);
    expect(buildRoomThreadPanelModel({ ...source, agentNameMap: { "private-agent-id": " Nova " } }, target, t)?.agentName).toBe("Nova");
    expect(buildRoomThreadPanelModel({ ...source, agentNameMap: { "private-agent-id": " " } }, target, t)?.agentName).toBe(model.agentName);
  });
});
