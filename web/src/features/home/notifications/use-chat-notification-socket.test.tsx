// INPUT: 全局 socket 的持久消息和轮次事件。
// OUTPUT: 已知会话本地更新，目录失效才查询服务端。
// POS: 通知传输与目录更新边界回归。
import { renderHook, act } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { extractAssistantReplyPreview } from "@/features/conversation/shared/message/message-content-model";
import { parseConversationMessage } from "@/lib/conversation/message-protocol";
import { buildChatNotificationDirectoryIndex } from "./chat-notification-directory";
import { useChatNotificationSocket } from "./use-chat-notification-socket";
const socket = vi.hoisted(() => ({ onMessage: (_value: unknown) => {}, refresh: vi.fn(), update: vi.fn(() => true) }));
vi.mock("@/lib/websocket", () => ({ useAppEventSubscription: vi.fn(), useWebSocket: (options: { onMessage: (value: unknown) => void }) => { socket.onMessage = options.onMessage; return { send: vi.fn(), state: "disconnected" }; } }));
vi.mock("@/lib/conversation/room-directory-events", () => ({ notifyRoomDirectoryUpdated: socket.refresh }));
vi.mock("../home-directory-resource", () => ({ applyHomeDirectoryRoomUpdate: socket.update }));
beforeEach(() => vi.clearAllMocks());
const event = { protocol_version: 1, room_id: "room-1", conversation_id: "conv-1", session_key: "session", timestamp: 1, event_type: "message", delivery_mode: "durable", data: { agent_id: "", message_id: "user-1", round_id: "round-1", role: "user", content: "Hello", timestamp: 1 } };
function setup(roomType: "dm" | "room" = "dm") {
  const completed = vi.fn();
  const directoryIndex = buildChatNotificationDirectoryIndex({ agents: [], rooms: [{ id: "room-1", room_type: roomType }], conversations: [{ conversation_id: "conv-1", room_id: "room-1", room_type: roomType, session_key: "session", title: "Chat", last_activity: "" }] });
  renderHook(() => useChatNotificationSocket({ directoryIndex, roomIdsKey: "", onCompletedMessage: completed }));
  return completed;
}
it("updates durable activity and replies locally without fetching each round", () => {
  const completed = setup();
  act(() => socket.onMessage(event));
  expect(socket.update).toHaveBeenCalledTimes(1);
  act(() => socket.onMessage({ ...event, event_type: "round_status", data: { status: "running" } }));
  act(() => socket.onMessage({ ...event, event_type: "text_delta", delivery_mode: "transient" }));
  expect(socket.update).toHaveBeenCalledTimes(1);
  act(() => socket.onMessage({ ...event, room_seq: 2, data: { ...event.data, agent_id: "agent-1", role: "assistant", is_complete: true, stop_reason: "end_turn", content: [{ type: "text", text: "Done" }] } }));
  expect(socket.update).toHaveBeenLastCalledWith(expect.objectContaining({ preview: "Done" }));
  expect(completed).toHaveBeenCalledTimes(1);
  expect(socket.refresh).not.toHaveBeenCalled();
});
it("excludes private group replies and projects public group messages", () => {
  setup("room");
  act(() => socket.onMessage(event));
  expect(socket.update).not.toHaveBeenCalled();
  act(() => socket.onMessage({ ...event, session_key: "room:room-1" }));
  expect(socket.update).toHaveBeenCalledTimes(1);
  expect(socket.refresh).not.toHaveBeenCalled();
});
it("clears rewritten previews and reconciles unknown conversations", () => {
  setup();
  act(() => socket.onMessage({ ...event, event_type: "session_resync_required", data: { reason: "history_rewrite" } }));
  expect(socket.update).toHaveBeenLastCalledWith({ roomId: "room-1", timestamp: 1, preview: "" });
  expect(socket.refresh).toHaveBeenCalledTimes(1);
  socket.update.mockReturnValueOnce(false);
  act(() => socket.onMessage(event));
  expect(socket.refresh).toHaveBeenCalledTimes(2);
});

it("extracts only the final text suffix and bounds Unicode previews", () => {
  const message = parseConversationMessage({ ...event.data, agent_id: "agent", role: "assistant", content: [{ type: "text", text: "Plan" }, { type: "tool_use", id: "tool", name: "read" }, { type: "text", text: "  Done\n\n now  " }] }, { sessionKey: "session" });
  if (!message || message.role !== "assistant") throw new Error("invalid fixture");
  expect(extractAssistantReplyPreview(message)).toBe("Done now");
  expect(Array.from(extractAssistantReplyPreview({ ...message, content: [{ type: "text", text: "😀".repeat(200) }] }))).toHaveLength(160);
});
it("resolves deleted sessions from directory event data", () => {
  setup();
  act(() => socket.onMessage({ protocol_version: 1, timestamp: 2, event_type: "directory_changed", data: { reason: "session_deleted", session_key: "session" } }));
  expect(socket.update).toHaveBeenLastCalledWith({ roomId: "room-1", timestamp: 2, preview: "" });
  expect(socket.refresh).toHaveBeenCalledTimes(1);
});
