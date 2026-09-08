// INPUT: 同名/缺名 Room、目录缺项及结构化 DM/Room Session。
// OUTPUT: 证明标签不暴露 ID，筛选和重排不交换名称，精确 Session 路由与来源徽标保留。
// POS: Automation 资源显示回归；候选权限与提交事务继续由既有合同覆盖。

import { describe, expect, it, vi } from "vitest";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";
import type { TaskFormDraft } from "../scheduled-task-dialog-types";
import { resolveTaskInheritedPermission, buildTaskDestinations, buildExecutionRoomOptions, buildRoomOptions, buildTaskDialogSessionData, buildTaskDialogDeliverySessionData } from "./task-dialog-resource-model";

function resource<T>(items: T[]) { return { items, error: null, loading: false, retry: vi.fn() }; }

function room(id: string, name: string, createdAt: string, paused = false): RoomAggregate {
  return {
    room: { id, name, created_at: createdAt, room_type: "room", description: "", skill_names: [], host_auto_reply_enabled: false, private_messages_enabled: false },
    members: [{ id: `member-${id}`, room_id: id, member_type: "agent", member_agent_id: "agent-private", participation_paused: paused }],
  };
}

function session(ref: string, kind: "dm" | "group" = "dm"): AgentSession {
  return {
    session_key: `agent:agent-private:ws:${kind}:${ref}`, agent_id: "agent-private", title: "重复标题",
    session_id: null, room_session_id: null, room_id: "room-private", conversation_id: ref,
    channel_type: "ws", chat_type: kind, status: "active", created_at: 0, last_activity_at: 0,
    message_count: 0, options: {},
  };
}

const form: TaskFormDraft = {
  dedicatedSessionKey: "", deliveryTargetType: "agent", enabled: true, executionKind: "agent", executionMode: "existing",
  expiresAt: "", instruction: "保留业务", permissionMode: "acceptEdits", replyMode: "selected",
  selectedAgentId: "agent-private", selectedDeliveryAgentId: "agent-private", selectedDeliveryPresenterAgentId: "",
  selectedDeliveryRoomId: "room-private", selectedReplySessionKey: "", selectedRoomId: "room-private",
  selectedSessionKey: "", targetType: "agent", taskName: "测试",
};

describe.each(["zh", "en"] as const)("scheduled selection labels in %s", (locale) => {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);

  it("combines complete destinations without internal delivery sessions or duplicate Room members", () => {
    const dm = session("dm");
    const member = session("group", "group");
    const internal = {...session("hidden"), options: {created_by: "automation_delivery"}};
    const options = buildTaskDestinations([dm, internal, member, {...member, agent_id: "another"}], [{value: "agent-private", label: "Agent"}], [{value: "room-private", label: "Room"}], t);
    expect(options).toHaveLength(2);
    expect(options[0]).toMatchObject({agentId: "agent-private", roomId: "", targetType: "agent", sessionKey: dm.session_key});
    expect(options[1]).toMatchObject({agentId: "", roomId: "room-private", targetType: "room"});
    expect(options[1].sessionKey).not.toBe(member.session_key);
  });

  it("shares Room labels across eligibility filters and directory reordering", () => {
    const a = room("room-private-a", " Research ", "2026-09-02");
    const b = room("room-private-b", "research", "2026-09-01", true);
    const unnamed = room("room-private-unnamed", " ", "invalid");
    const dm = room("dm-private", "Research", "2026-08-01");
    dm.room.room_type = "dm";
    const rooms = [a, b, unnamed, dm];
    const options = buildRoomOptions(rooms, t);
    expect(options).toEqual([
      { value: a.room.id, label: "2 · Research" }, { value: b.room.id, label: "1 · research" },
      { value: unnamed.room.id, label: `1 · ${t("room.untitled_collaboration")}` },
    ]);
    expect(buildRoomOptions([...rooms].reverse(), t)).toEqual([...options].reverse());
    expect(buildExecutionRoomOptions(rooms, t)).toEqual([options[0], options[2]]);
    expect(a.room.name).toBe(" Research ");
  });

  it("distinguishes same-title DM sessions without leaking a missing Agent name", () => {
    const sessions = [session("b"), session("a")];
    const options = buildTaskDialogSessionData("agent", { agentSessions: resource(sessions), roomContexts: resource([]) }, t).options;
    expect(options.map((option) => option.label)).toEqual(["2 · 重复标题", "1 · 重复标题"]);
    expect(options.map((option) => option.value)).toEqual(sessions.map((item) => item.session_key));
    expect(buildTaskDialogDeliverySessionData(form, resource([...sessions].reverse()), t).options).toEqual([...options].reverse());
    const unnamed = buildTaskDialogSessionData("agent", { agentSessions: resource(sessions.map((item) => ({ ...item, title: " " }))), roomContexts: resource([]) }, t).options;
    expect(unnamed.map((option) => option.value)).toEqual(options.map((option) => option.value));
    expect(unnamed.map((option) => option.label)).toEqual([`2 · ${t("capability.scheduled_dialog_unnamed_session")}`, `1 · ${t("capability.scheduled_dialog_unnamed_session")}`]);
  });

  it("keeps shared Room routes, deduplication and badges when directory names disappear", () => {
    const sessions = [session("b", "group"), session("a", "group")];
    const group = room("room-private", " ", "2026-09-01");
    const contexts: RoomContextAggregate[] = sessions.map((item) => ({
      ...group, member_agents: [],
      conversation: { id: item.conversation_id!, room_id: group.room.id, conversation_type: "room", title: item.title },
      sessions: [{ id: item.session_key, conversation_id: item.conversation_id!, agent_id: item.agent_id, runtime_id: "runtime", version_no: 1, branch_key: "", is_primary: true, options: {}, status: "active" }],
    }));
    const deliveryForm = { ...form, deliveryTargetType: "room" as const };
    const execution = buildTaskDialogSessionData("room", { agentSessions: resource([]), roomContexts: resource(contexts) }, t).options;
    const delivery = buildTaskDialogDeliverySessionData(deliveryForm, resource([...sessions, { ...sessions[0], agent_id: "other-member" }]), t).options;
    expect(delivery).toEqual(execution);
    expect(delivery.map((option) => option.label)).toEqual(["2 · 重复标题", "1 · 重复标题"]);
    expect(delivery.map(({ value, sessionKey, badge }) => ({ value, sessionKey, badge }))).toEqual([
      { value: "room:group:b", sessionKey: "room:group:b", badge: "Room" },
      { value: "room:group:a", sessionKey: "room:group:a", badge: "Room" },
    ]);
    group.room.name = "2 · Research";
    const renamed = buildTaskDialogSessionData("room", { agentSessions: resource([]), roomContexts: resource(contexts) }, t).options;
    expect(renamed).toEqual(execution);
  });
});

it("previews session permissions before Agent defaults and never guesses on a failed read", () => {
  const agents = resource([{agent_id: "agent-private", name: "Agent", workspace_path: "", created_at: 0, status: "active", options: {permission_mode: "auto"}} satisfies Agent]);
  const dm = {...session("permission"), options: {permission_mode: "default"}};
  const draft = {...form, selectedSessionKey: dm.session_key};
  expect(resolveTaskInheritedPermission(draft, agents, resource([dm]), resource([]), "")).toBe("default");
  expect(resolveTaskInheritedPermission({...draft, executionMode: "temporary"}, agents, resource([dm]), resource([]), "")).toBe("auto");
  expect(resolveTaskInheritedPermission(draft, agents, {...resource([dm]), error: "failed"}, resource([]), "")).toBeNull();
  const member = {id: "member", conversation_id: "group", agent_id: "agent-private", runtime_id: "runtime", version_no: 1, branch_key: "", is_primary: false, options: {permission_mode: "default"}, status: "active"};
  const context: RoomContextAggregate = {...room("room-private", "Research", "2026-09-01"), member_agents: agents.items, conversation: {id: "group", room_id: "room-private", conversation_type: "room"}, sessions: [member, {...member, id: "primary", is_primary: true, options: {permission_mode: "bypassPermissions"}}]};
  expect(resolveTaskInheritedPermission({...draft, targetType: "room", selectedAgentId: "", selectedSessionKey: "room:group:group"}, agents, resource([]), resource([context]), "agent-private")).toBe("bypassPermissions");
});
