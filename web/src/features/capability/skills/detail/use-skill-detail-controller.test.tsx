// INPUT: Same-frame Agent toggles, pending writes and locale changes.
// OUTPUT: A single write remains protected while language and read recovery actions change.
// POS: Skill detail command admission regression.
import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { SkillAgentBinding } from "@/types/capability/skill";
import { useSkillDetailController } from "./use-skill-detail-controller";
const api = vi.hoisted(() => ({ detail: vi.fn(), bindings: vi.fn(), toggle: vi.fn(), t: (key: string) => key }));
vi.mock("@/lib/api/capability/skill-api", () => ({ getSkillDetailApi: api.detail, getSkillAgentsApi: api.bindings, setAgentSkillEnabledApi: api.toggle }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: api.t }) }));
it("admits one exact toggle and keeps locale changes and retries from restarting the read", async () => {
  const binding = { agent_id: "agent-1", enabled: false, available: true } as SkillAgentBinding;
  api.detail.mockResolvedValue({ name: "skill-1", scope: "any", locked: false });
  api.bindings.mockResolvedValue([binding]);
  let finish!: (value: { enabled_for_agent: boolean }) => void;
  api.toggle.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const options = { skillName: "skill-1", deleteSkill: vi.fn(), updateSkill: vi.fn(), onDeleted: vi.fn(), onAgentBindingChanged: vi.fn() };
  const { result, rerender } = renderHook(() => useSkillDetailController(options));
  await waitFor(() => expect(result.current.agentsLoading).toBe(false));
  let pending!: Promise<void>;
  act(() => { pending = result.current.toggleAgent(binding); void result.current.toggleAgent(binding); });
  expect(api.toggle).toHaveBeenCalledTimes(1);
  api.t = (key) => `translated:${key}`;
  rerender();
  await act(async () => { await result.current.retryBindings(); });
  expect(api.detail).toHaveBeenCalledTimes(1);
  expect(api.bindings).toHaveBeenCalledTimes(1);
  await act(async () => { finish({ enabled_for_agent: true }); await pending; });
  expect(result.current.agentBindings[0].enabled).toBe(true);
});

it("requires explicit new intent when a read cannot prove an unknown toggle", async () => {
  api.toggle.mockReset();
  const binding = { agent_id: "agent-2", enabled: false, available: true } as SkillAgentBinding;
  api.detail.mockResolvedValue({ name: "skill-2", scope: "any", locked: false });
  api.bindings.mockResolvedValue([binding]);
  api.toggle.mockRejectedValueOnce(new Error("offline")).mockResolvedValue({ enabled_for_agent: true });
  const options = { skillName: "skill-2", deleteSkill: vi.fn(), updateSkill: vi.fn(), onDeleted: vi.fn(), onAgentBindingChanged: vi.fn() };
  const { result } = renderHook(() => useSkillDetailController(options));
  await waitFor(() => expect(result.current.agentsLoading).toBe(false));
  await act(async () => { await result.current.toggleAgent(binding); });
  await act(async () => { await result.current.retryBindings(); });
  expect(result.current.toggleFailures["agent-2"].canStartNewIntent).toBe(true);
  await act(async () => { await result.current.toggleAgent(binding); });
  expect(api.toggle).toHaveBeenCalledTimes(1);
  act(() => { result.current.startNewToggleIntent("agent-2"); });
  await act(async () => { await result.current.toggleAgent(binding); });
  expect(api.toggle).toHaveBeenCalledTimes(2);
});
