import { UserRound } from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

const HOME_ONBOARDING_ROLES = [
  "学生",
  "产品经理",
  "研发",
  "运营",
  "解决方案及售前",
] as const;

interface HomeOnboardingRoleCardProps {
  onSelect: (role: string) => void;
}

export function HomeOnboardingRoleCard({
  onSelect,
}: HomeOnboardingRoleCardProps) {
  return (
    <section
      aria-labelledby="home-onboarding-role-card-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_12%,transparent),transparent_58%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <UserRound className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0">
            <h3
              className="text-[15px] font-semibold text-(--text-strong)"
              id="home-onboarding-role-card-title"
            >
              选择你的角色
            </h3>
            <p className="mt-1 text-[13px] leading-6 text-(--text-muted)">
              选好后，接入的对话模型会根据你的角色承接后续 Nexus 引导。
            </p>
          </div>
        </div>
      </div>

      <div className="grid gap-2.5 px-5 py-5 sm:grid-cols-2 sm:px-6">
        {HOME_ONBOARDING_ROLES.map((role) => (
          <button
            className={getUiButtonClassName(
              { size: "md", tone: "default", variant: "surface" },
              "justify-start rounded-xl px-4 py-3 text-left",
            )}
            key={role}
            onClick={() => onSelect(role)}
            type="button"
          >
            {role}
          </button>
        ))}
      </div>
    </section>
  );
}
