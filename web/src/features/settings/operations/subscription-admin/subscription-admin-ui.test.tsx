// INPUT: Subscription 账号/套餐视图模型及稳定的本地化文本。
// OUTPUT: 证明 Operations 订阅视图复用 Settings Card、Typography、Badge 与 Resource State。
// POS: Subscription 展示合同测试；请求并发与 mutation 对账由 controller/model 测试负责。

import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import type { AccountViewModel, PlanViewModel } from "./subscription-admin-model";
import { createEmptyPlanDraft } from "./subscription-admin-model";
import { SubscriptionAccountView } from "./subscription-account-view";
import { SubscriptionPlanView } from "./subscription-plan-view";

function renderWithI18n(children: ReactNode) {
  return render(
    <I18N_CONTEXT.Provider
      value={{
        locale: "zh",
        setLocale: vi.fn(),
        t: (key) => key,
      }}
    >
      {children}
    </I18N_CONTEXT.Provider>,
  );
}

const accountModel: AccountViewModel = {
  accounts: [{
    created_at: "2026-09-01T00:00:00Z",
    display_name: "Ada",
    message_count: 8,
    monthly_token_limit: 1000,
    owner_user_id: "owner-1",
    period_end: "2026-09-30T00:00:00Z",
    period_start: "2026-09-01T00:00:00Z",
    plan_key: "pro",
    plan_name: "Pro",
    role: "owner",
    session_count: 3,
    updated_at: "2026-09-01T00:00:00Z",
    used_percent: 20,
    used_tokens: 200,
    user_status: "active",
    username: "ada",
  }],
  drafts: { "owner-1": { planKey: "pro" } },
  loading: false,
  mutationPending: false,
  mutationsBlocked: false,
  periodEnd: "2026-09-30T00:00:00Z",
  periodStart: "2026-09-01T00:00:00Z",
  plans: [{
    display_name: "Pro",
    monthly_token_limit: 1000,
    notes: "",
    plan_key: "pro",
    sort_order: 1,
    status: "active",
  }],
  savingOwnerUserId: null,
  summary: { accountCount: 1, planCount: 1, usedTokens: 200 },
};

describe("Subscription Admin UI ownership", () => {
  it("renders account identity and role through semantic typography and Badge", () => {
    const { container } = renderWithI18n(
      <SubscriptionAccountView
        model={accountModel}
        onChangeDraft={vi.fn()}
        onRefresh={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByText("Ada").className).toContain("ui-type-section-title");
    expect(screen.getByText("owner").className).toContain("radius-control-xs");
    expect(screen.getByText("settings.subscription.plan").className).toContain("ui-type-caption");
    expect(container.querySelectorAll(".surface-radius-md").length).toBeGreaterThanOrEqual(2);
  });

  it("renders plan loading through the shared resource-state contract", () => {
    const model: PlanViewModel = {
      creating: false,
      drafts: {},
      loading: true,
      mutationPending: false,
      mutationsBlocked: false,
      newPlanDraft: createEmptyPlanDraft(),
      plans: [],
      savingPlanKey: null,
    };

    const { container } = renderWithI18n(
      <SubscriptionPlanView
        model={model}
        onChangeDraft={vi.fn()}
        onChangeNewDraft={vi.fn()}
        onCreate={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(container.querySelector('[data-resource-state="loading"]')).toBeTruthy();
    expect(screen.getByText("settings.subscription.loading").className).toContain("ui-type-object-title");
  });
});
