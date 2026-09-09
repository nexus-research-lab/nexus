// INPUT: Source writes with unknown results and later list reads.
// OUTPUT: Read success alone never unlocks an opaque write; explicit new intent is required.
// POS: Source recovery controller regression.
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";
import { useExternalSkillSources } from "./use-external-skill-sources";

const api = vi.hoisted(() => ({ update: vi.fn(), read: vi.fn(), t: (key: string) => key }));
vi.mock("@/lib/api/capability/skill-api", () => ({ createExternalSkillSourceApi: vi.fn(), deleteExternalSkillSourceApi: vi.fn(), updateExternalSkillSourceApi: api.update, listExternalSkillSourcesApi: api.read }));
vi.mock("@/shared/i18n/i18n-context", () => ({ useI18n: () => ({ t: api.t }) }));
it("keeps unknown writes locked after reads until the user explicitly starts a new intent", async () => {
  const source = { source_id: "private-1", name: "Private", enabled: true } as ExternalSkillSourceInfo;
  api.update.mockRejectedValueOnce(new Error("offline")).mockResolvedValue(undefined);
  api.read.mockResolvedValue([source]);
  const feedback = { clear: vi.fn(), report: vi.fn(), start: vi.fn(), success: vi.fn(), error: vi.fn(), warning: vi.fn() };
  const { result } = renderHook(() => useExternalSkillSources({ active: false, feedback }));
  await act(async () => { await result.current.toggle(source, false); });
  await act(async () => { feedback.report.mock.lastCall![0].action.onClick(); });
  expect(feedback.report.mock.lastCall![0].action.label).toBe("capability.skill_operation_new_intent_action");
  await act(async () => { await result.current.toggle(source, false); });
  expect(api.update).toHaveBeenCalledTimes(1);
  await act(async () => { feedback.report.mock.lastCall![0].action.onClick(); });
  await act(async () => { await result.current.toggle(source, false); });
  expect(api.update).toHaveBeenCalledTimes(2);
});
