// INPUT: 授权事件与重复提交/取消控制事件。
// OUTPUT: 一个授权 ACK 返回前仅发送一个控制帧，其他 flow 的 ACK 不能解锁。
// POS: Presenter 同步互斥与精确 flow 回归。
import { act, cleanup, render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ChannelAuthorizationPresenter } from "./channel-authorization-presenter";
import type { ComponentProps } from "react";
import type { ChannelAuthorizationDialog } from "./channel-authorization-dialog";

const state = vi.hoisted(() => ({ send: vi.fn(() => ({ disposition: "sent" })), onMessage: (_: unknown) => {}, props: null as ComponentProps<typeof ChannelAuthorizationDialog> | null }));
vi.mock("@/lib/websocket", () => ({ useWebSocket: ({ onMessage }: { onMessage: (value: unknown) => void }) => { state.onMessage = onMessage; return { send: state.send }; } }));
vi.mock("./channel-authorization-dialog", () => ({ ChannelAuthorizationDialog: (props: ComponentProps<typeof ChannelAuthorizationDialog>) => { state.props = props; return null; } }));
afterEach(cleanup);
const event = (event_type: string, data: unknown) => ({ event_type, data, protocol_version: 1, timestamp: Date.now() });
it("does not submit or cancel twice while waiting for the exact flow ACK", () => {
  render(<I18nProvider><ChannelAuthorizationPresenter /></I18nProvider>);
  act(() => state.onMessage(event("channel_authorization", { flow_id: "flow", presentation_token: "token", channel_type: "telegram", kind: "verification_code", account_binding: "owner-channel", prompt: "Code", expires_at: "2099-01-01T00:00:00Z" })));
  act(() => { state.props!.onSubmitCode("123"); state.props!.onSubmitCode("123"); state.props!.onCancelAuthorization(); });
  expect(state.send).toHaveBeenCalledTimes(1);
  act(() => state.onMessage(event("channel_authorization_result", { flow_id: "other", accepted: true })));
  act(() => state.props!.onCancelAuthorization());
  expect(state.send).toHaveBeenCalledTimes(1);
  expect(state.props!.busy).toBe(true);
});
