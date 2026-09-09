// INPUT: 套餐草稿、按需展开操作与写事务禁用状态。
// OUTPUT: 折叠保留编辑值，保存只作用于精确套餐，未对账时禁止提交。
// POS: 运营视图交互回归；不请求后台或复制事务实现。
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { SubscriptionPlanView } from "./subscription-admin/subscription-plan-view";
import { createEmptyPlanDraft, createPlanDraft, type PlanViewModel } from "./subscription-admin/subscription-admin-model";

const plan = { plan_key: "research", display_name: "Research", status: "active", monthly_token_limit: 1_000_000, notes: "", sort_order: 0 };
afterEach(cleanup);

it("套餐编辑按需展开，保留草稿并遵守 mutation 锁", async () => {
  const user = userEvent.setup();
  const save = vi.fn(async () => {});
  const change = vi.fn();
  const model: PlanViewModel = { creating: false, drafts: { research: createPlanDraft(plan) }, loading: false, mutationPending: false, mutationsBlocked: false, newPlanDraft: createEmptyPlanDraft(), plans: [plan], savingPlanKey: null };
  const view = () => <I18nProvider><SubscriptionPlanView model={model} onChangeDraft={change} onChangeNewDraft={vi.fn()} onCreate={vi.fn(async () => {})} onSave={save} /></I18nProvider>;
  const { container, rerender } = render(view());
  const detail = container.querySelectorAll("details")[1];
  expect(detail.open).toBe(false);
  await user.click(detail.querySelector("summary")!);
  const name = within(detail).getAllByRole("textbox")[0];
  await user.clear(name);
  await user.type(name, "Team");
  expect(change).toHaveBeenCalledWith("research", expect.objectContaining({ displayName: expect.any(String) }));
  model.drafts.research.displayName = "Team";
  rerender(view());
  await user.click(detail.querySelector("summary")!);
  await user.click(detail.querySelector("summary")!);
  expect((within(detail).getAllByRole("textbox")[0] as HTMLInputElement).value).toBe("Team");
  await user.click(within(detail).getByRole("button", { name: /保存|Save/ }));
  expect(save).toHaveBeenCalledExactlyOnceWith("research");
  model.mutationsBlocked = true;
  rerender(view());
  expect((screen.getByRole("button", { name: /保存|Save/ }) as HTMLButtonElement).disabled).toBe(true);
});
