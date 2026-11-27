// INPUT: 个人用量汇总与密码提交状态。
// OUTPUT: 验证用量明细不重复、每日图表与密码表单按需展开，本机单用户不显示所有者徽标。
// POS: 个人页重排的局部交互回归，不调用账户 API。
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { MESSAGES, type TranslationKey } from "@/shared/i18n/messages";
import { PersonalUsageHeatmap, PersonalUsageHistory } from "./personal-usage-history";
import { PersonalPasswordSection } from "./personal-password-section";
import { PersonalTokenUsageSection } from "./personal-token-usage-section";

vi.mock("@/shared/i18n/i18n-context", () => ({
  useI18n: () => ({ locale: "zh", t: (key: TranslationKey, params: Record<string, string | number> = {}) => Object.entries(params).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.zh[key] as string) }),
}));

it("compacts heatmap tooltips and chart details", async () => {
  const user = userEvent.setup();
  const date = new Date().toISOString().slice(0, 10);
  const days = [{ date, input_tokens: 110_130_331, output_tokens: 68_764, cache_tokens: 983_680, total_tokens: 111_182_775 }];
  render(<><PersonalUsageHeatmap days={days} /><PersonalUsageHistory days={days} /></>);
  const heatmap = within(screen.getByRole("region", { name: "Token 活动" }));
  await user.hover(heatmap.getByRole("button", { name: `${date}: ${days[0].total_tokens.toLocaleString()} Token` }));
  expect((await screen.findByRole("tooltip")).textContent).toContain("使用了 1.1亿 个 Token");
  expect(screen.getByText("输入 Token · 1.1亿")).toBeTruthy();
  expect(screen.getByText("输出 Token · 6.9万")).toBeTruthy();
  expect(screen.getByText("缓存 Token · 98.4万")).toBeTruthy();
});

it("keeps a single usage breakdown and expands the password form before submitting", async () => {
  const onSubmit = vi.fn();
  const user = userEvent.setup();
  render(<>
    <PersonalTokenUsageSection usage={{ input_tokens: 100, output_tokens: 200, cache_creation_input_tokens: 50, cache_read_input_tokens: 50, total_tokens: 400, quota_limit_tokens: null, session_count: 2, message_count: 3, updated_at: "2026-09-09T00:00:00Z" }} />
    <PersonalPasswordSection canChange canSubmit hasInput isSubmitting={false} mutationBlocked={false}
      draft={{ currentPassword: "old-password", newPassword: "new-password", confirmPassword: "new-password" }}
      onFieldChange={vi.fn()} onSubmit={onSubmit} validationError={null} />
  </>);
  expect(screen.getAllByText("输入 Token")).toHaveLength(1);
  expect(screen.getAllByText("输出 Token")).toHaveLength(1);
  expect(screen.getAllByText("缓存 Token")).toHaveLength(1);
  expect(screen.getByText("400")).toBeTruthy();
  expect(screen.queryByText("额度上限")).toBeNull();
  await user.hover(screen.getByText("400"));
  expect((await screen.findByRole("tooltip")).textContent).toContain("400");
  await user.unhover(screen.getByText("400"));
  const disclosure = document.querySelector("details")!;
  expect(disclosure.open).toBe(false);
  await user.click(disclosure.querySelector("summary")!);
  expect(disclosure.open).toBe(true);
  expect(screen.getByLabelText("当前密码")).toBeTruthy();
  await user.click(screen.getByRole("button", { name: "修改密码" }));
  expect(onSubmit).toHaveBeenCalledOnce();
});


it("associates password guidance with each form independently and exposes saving", () => {
  const common = { canChange: true, canSubmit: false, hasInput: true, mutationBlocked: false,
    draft: { currentPassword: "old", newPassword: "new", confirmPassword: "new" },
    onFieldChange: vi.fn(), onSubmit: vi.fn() };
  const { container } = render(<>
    <PersonalPasswordSection {...common} isSubmitting={false} validationError="First error" />
    <PersonalPasswordSection {...common} isSubmitting validationError="Second error" />
  </>);
  const forms = container.querySelectorAll("form");
  forms.forEach((form, index) => {
    const inputs = form.querySelectorAll("input");
    inputs.forEach(input => {
      const helper = document.getElementById(input.getAttribute("aria-describedby")!);
      expect(form.contains(helper)).toBe(true);
      expect(helper?.textContent).toBe(index === 0 ? "First error" : "Second error");
    });
    expect(form.getAttribute("aria-busy")).toBe(String(index === 1));
  });
});

it("hides the owner badge for the local single-user profile but keeps it for accounts", async () => {
  const { buildPersonalProfilePresentation } = await import("./personal-settings-model");
  const t = (key: TranslationKey) => MESSAGES.zh[key];
  const profile = (auth_method: string, plan_name?: string) => ({
    user: { user_id: "u", username: "local", display_name: "Local", role: "owner", avatar: "", auth_method },
    token_usage: { cache_creation_input_tokens: 0, cache_read_input_tokens: 0, input_tokens: 0, message_count: 0, output_tokens: 0, quota_limit_tokens: null, session_count: 0, total_tokens: 0, updated_at: "" },
    subscription: plan_name ? { plan_name } as never : null,
    can_change_password: false,
    can_update_profile: true,
  });
  expect(buildPersonalProfilePresentation(profile("local"), t).showIdentityBadge).toBe(false);
  expect(buildPersonalProfilePresentation(profile("local", "Pro"), t).showIdentityBadge).toBe(true);
  expect(buildPersonalProfilePresentation(profile("password"), t).showIdentityBadge).toBe(true);
});
