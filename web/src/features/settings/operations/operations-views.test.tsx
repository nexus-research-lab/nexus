// INPUT: 套餐草稿、按需展开操作与写事务禁用状态。
// OUTPUT: 直接编辑保留草稿值，保存只作用于精确套餐，未对账时禁止提交。
// POS: 运营视图交互回归；不请求后台或复制事务实现。
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { SubscriptionAccountView } from "./subscription-admin/subscription-account-view";
import { SubscriptionPlanView } from "./subscription-admin/subscription-plan-view";
import { buildPlanPayload, createEmptyPlanDraft, createPlanDraft, type AccountViewModel, type PlanViewModel } from "./subscription-admin/subscription-admin-model";

const plan = { plan_key: "research", display_name: "Research", status: "active", monthly_token_limit: 1_000_000, notes: "", sort_order: 0 };
afterEach(cleanup);

it("套餐直接编辑，保留草稿并遵守 mutation 锁", async () => {
  const user = userEvent.setup();
  const save = vi.fn(async () => {});
  const change = vi.fn();
  const model: PlanViewModel = { creating: false, drafts: { research: createPlanDraft(plan) }, loading: false, mutationPending: false, mutationsBlocked: false, newPlanDraft: createEmptyPlanDraft(), plans: [plan], savingPlanKey: null };
  const view = () => <I18nProvider><SubscriptionPlanView model={model} onChangeDraft={change} onChangeNewDraft={vi.fn()} onCreate={vi.fn(async () => {})} onSave={save} /></I18nProvider>;
  const { rerender } = render(view());
  const detail = screen.getByRole("article", { name: "Research" });
  const name = within(detail).getAllByRole("textbox")[0];
  await user.clear(name);
  await user.type(name, "Team");
  expect(change).toHaveBeenCalledWith("research", expect.objectContaining({ displayName: expect.any(String) }));
  model.drafts.research.displayName = "Team";
  rerender(view());
  expect((within(detail).getAllByRole("textbox")[0] as HTMLInputElement).value).toBe("Team");
  await user.click(within(detail).getByRole("button", { name: /保存|Save/ }));
  expect(save).toHaveBeenCalledExactlyOnceWith("research");
  model.mutationsBlocked = true;
  rerender(view());
  expect((screen.getByRole("button", { name: /保存|Save/ }) as HTMLButtonElement).disabled).toBe(true);
});


it("未对账订阅写入仍允许刷新权威快照，实际请求进行中禁用刷新", async () => {
  const user = userEvent.setup();
  const refresh = vi.fn(async () => {});
  const model: AccountViewModel = { accounts: [], drafts: {}, loading: false, mutationPending: false, mutationsBlocked: false, periodStart: "", periodEnd: "", plans: [], savingOwnerUserId: null };
  const view = () => <I18nProvider><SubscriptionAccountView model={model} onChangeDraft={vi.fn()} onRefresh={refresh} onSave={vi.fn(async () => {})} /></I18nProvider>;
  const { rerender } = render(view());
  expect(screen.queryByRole("button", { name: /刷新|Refresh/ })).toBeNull();
  model.mutationsBlocked = true;
  rerender(view());
  await user.click(screen.getByRole("button", { name: /刷新|Refresh/ }));
  expect(refresh).toHaveBeenCalledTimes(1);
  model.mutationPending = true;
  rerender(view());
  expect((screen.getByRole("button", { name: /刷新|Refresh/ }) as HTMLButtonElement).disabled).toBe(true);
});


it("新增套餐弹窗隐藏随机 Key，成功重置草稿后关闭，改名保留原 Key", async () => {
  const user = userEvent.setup();
  const draft = createEmptyPlanDraft();
  expect(draft.planKey).toMatch(/^[a-f0-9-]{36}$/);
  expect(createEmptyPlanDraft().planKey).not.toBe(draft.planKey);
  expect(buildPlanPayload(plan.plan_key, { ...createPlanDraft(plan), displayName: "Renamed" })?.plan_key).toBe(plan.plan_key);
  const model: PlanViewModel = { creating: false, drafts: {}, loading: false, mutationPending: false, mutationsBlocked: false, newPlanDraft: draft, plans: [], savingPlanKey: null };
  const create = vi.fn(async () => {});
  const changeNewDraft = vi.fn((patch) => Object.assign(draft, patch));
  const view = () => <I18nProvider><SubscriptionPlanView model={model} onChangeDraft={vi.fn()} onChangeNewDraft={changeNewDraft} onCreate={create} onSave={vi.fn(async () => {})} /></I18nProvider>;
  const { rerender } = render(view());
  await user.click(screen.getByRole("button", { name: /新增套餐|Add plan/ }));
  const dialog = screen.getByRole("dialog");
  expect(within(dialog).queryByText(/套餐 Key|Plan key/)).toBeNull();
  await user.type(within(dialog).getByRole("textbox", { name: /备注|Notes/ }), "A");
  expect(changeNewDraft).toHaveBeenCalledWith({ notes: "A" });
  expect(buildPlanPayload(draft.planKey, draft)?.notes).toBe("A");
  await user.click(within(dialog).getByRole("button", { name: /新增套餐|Add plan/ }));
  expect(create).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("dialog")).toBeTruthy();
  model.newPlanDraft = createEmptyPlanDraft();
  rerender(view());
  expect(screen.queryByRole("dialog")).toBeNull();
});
