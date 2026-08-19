import { ArrowRight, KeyRound, Settings2 } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { beginHomeOnboardingProviderConfiguration } from "./home-agent-onboarding";

const PROVIDER_CONFIGURATION_STEPS = [
  "选择一个 LLM Provider，或添加自定义 Provider。",
  "填写 API Token 和接口地址，然后保存配置。",
  "同步并启用至少一个模型，再执行连通性测试。",
] as const;

export function HomeOnboardingProviderConfigCard() {
  const navigate = useNavigate();

  return (
    <section
      aria-labelledby="home-onboarding-provider-card-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_12%,transparent),transparent_58%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <KeyRound className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0">
            <h3
              className="text-[15px] font-semibold text-(--text-strong)"
              id="home-onboarding-provider-card-title"
            >
              连续两次校验未通过
            </h3>
            <p className="mt-1 text-[13px] leading-6 text-(--text-muted)">
              自动配置没有成功，你可以在 Nexus 的模型配置页手动完成接入：
            </p>
          </div>
        </div>
      </div>

      <div className="px-5 py-5 sm:px-6">
        <ol className="space-y-3">
          {PROVIDER_CONFIGURATION_STEPS.map((step, index) => (
            <li
              className="flex items-start gap-3 text-[13px] leading-6 text-(--text-muted)"
              key={step}
            >
              <span className="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-[color-mix(in_srgb,var(--primary)_12%,transparent)] text-[11px] font-semibold text-primary">
                {index + 1}
              </span>
              <span>{step}</span>
            </li>
          ))}
        </ol>

        <button
          className={getUiButtonClassName(
            { size: "md", tone: "primary", variant: "solid" },
            "group mt-5 w-full justify-center rounded-xl sm:w-auto",
          )}
          onClick={() => {
            beginHomeOnboardingProviderConfiguration();
            navigate(AppRouteBuilders.settings("providers"));
          }}
          type="button"
        >
          <Settings2 className="h-4 w-4" />
          前往模型配置
          <ArrowRight className="h-4 w-4 transition-transform duration-200 group-hover:translate-x-0.5" />
        </button>
      </div>
    </section>
  );
}
