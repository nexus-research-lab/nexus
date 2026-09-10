// INPUT: Durable user messages and round lifecycle events from the global socket.
// OUTPUT: Directory invalidation occurs before an assistant completion, never on token deltas.
// POS: Notification transport regression with mocked socket transport.
import { renderHook, act } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { useChatNotificationSocket } from "./use-chat-notification-socket";
const socket = vi.hoisted(() => ({ onMessage: (_value: unknown) => {}, refresh: vi.fn() }));
vi.mock("@/lib/websocket", () => ({ useAppEventSubscription: vi.fn(), useWebSocket: (options: { onMessage: (value: unknown) => void }) => { socket.onMessage = options.onMessage; return { send: vi.fn(), state: "disconnected" }; } }));
vi.mock("@/lib/conversation/room-directory-events", () => ({ notifyRoomDirectoryUpdated: socket.refresh }));
it("refreshes durable user activity and round state without treating either as a completed reply", () => {
  const completed = vi.fn();
  renderHook(() => useChatNotificationSocket({ directoryIndex: { agentsById: new Map(), conversationsById: new Map(), conversationsBySessionKey: new Map(), roomsById: new Map(), sessionTargetKeysByRoomId: new Map() }, roomIdsKey: "", onCompletedMessage: completed }));
  const event = { protocol_version: 1, session_key: "session", timestamp: 1, event_type: "message", delivery_mode: "durable", data: { agent_id: "", message_id: "user-1", round_id: "round-1", role: "user", content: "Hello", timestamp: 1 } };
  act(() => socket.onMessage(event));
  expect(socket.refresh).toHaveBeenCalledTimes(1);
  act(() => socket.onMessage({ ...event, event_type: "round_status", data: { status: "running" } }));
  expect(socket.refresh).toHaveBeenCalledTimes(2);
  act(() => socket.onMessage({ ...event, event_type: "text_delta", delivery_mode: "transient" }));
  expect(socket.refresh).toHaveBeenCalledTimes(2);
  expect(completed).not.toHaveBeenCalled();
});
