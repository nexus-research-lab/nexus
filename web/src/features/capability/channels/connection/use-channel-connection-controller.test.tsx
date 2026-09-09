// INPUT: A QR-enabled channel and an active verification challenge.
// OUTPUT: Prevents a second configuration save or login while verification is pending.
// POS: Connection-to-login admission regression.
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { ChannelConfigView } from "@/lib/api/capability/channel-api";
import { useChannelConnectionController } from "./use-channel-connection-controller";

const api = vi.hoisted(() => ({ save: vi.fn(), start: vi.fn() }));
vi.mock("@/lib/api/capability/channel-api", () => ({
  upsertChannelConfigApi: api.save,
  startChannelLoginApi: api.start,
  getCurrentChannelLoginApi: vi.fn(),
  getChannelLoginApi: vi.fn(),
  submitChannelLoginVerifyCodeApi: vi.fn(),
  deleteChannelAccountApi: vi.fn(),
  deleteChannelConfigApi: vi.fn(),
  listChannelsApi: vi.fn(),
}));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
it("keeps the main save locked while the QR login waits for verification", async () => {
  const item = { channel_type: "weixin-personal", agent_id: "agent-1", supports_qr_code: true, configured: true, has_credentials: false, public_config: {}, credential_fields: [] } as unknown as ChannelConfigView;
  api.save.mockResolvedValue(item);
  api.start.mockResolvedValue({ login_id: "login-1", status: "verify_code_required" });
  const { result } = renderHook(() => useChannelConnectionController({ agents: [], item, onClose: vi.fn(), onSaved: vi.fn(), onDeleted: vi.fn() }));
  await act(async () => { await result.current.saveChannel(); });
  expect(result.current.loginRunning).toBe(true);
  await act(async () => { expect(await result.current.saveChannel()).toBe(false); });
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(api.start).toHaveBeenCalledTimes(1);
});
