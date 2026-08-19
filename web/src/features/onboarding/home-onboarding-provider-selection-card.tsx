import { ArrowRight, LoaderCircle, Settings2, Sparkles } from "lucide-react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { ProviderIcon } from "@/features/settings/provider-settings/components/provider-settings-icon";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import { cn } from "@/shared/ui/class-name";

import { beginHomeOnboardingProviderConfiguration } from "./home-agent-onboarding";
import type {
  HomeOnboardingProviderChoice,
} from "./home-onboarding-provider";

interface HomeOnboardingProviderSelectionCardProps {
  choices: HomeOnboardingProviderChoice[];
  error: string | null;
  loading: boolean;
  onRetry: () => void;
  onSelect: (choice: HomeOnboardingProviderChoice) => void;
}

export function HomeOnboardingProviderSelectionCard({
  choices,
  error,
  loading,
  onRetry,
  onSelect,
}: HomeOnboardingProviderSelectionCardProps) {
  const navigate = useNavigate();

  return (
    <section
      aria-labelledby="home-onboarding-provider-selection-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_14%,transparent),transparent_62%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <Sparkles className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0">
            <h3
              className="text-[15px] font-semibold text-(--text-strong)"
              id="home-onboarding-provider-selection-title"
            >
              先选择模型厂商
            </h3>
            <p className="mt-1 text-[13px] leading-6 text-(--text-muted)">
              我会根据你的选择使用对应接口校验 Token，不会再自动判断厂商。
            </p>
          </div>
        </div>
      </div>

      <div className="px-5 py-5 sm:px-6">
        {loading ? (
          <div className="flex min-h-28 items-center justify-center gap-2 text-[13px] text-(--text-muted)">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            正在读取可用厂商…
          </div>
        ) : null}

        {!loading && error ? (
          <div className="flex min-h-28 flex-col items-center justify-center gap-3 text-center">
            <p className="text-[13px] leading-6 text-(--text-muted)">
              {error}
            </p>
            <button
              className={getUiButtonClassName({
                size: "sm",
                tone: "default",
                variant: "surface",
              })}
              onClick={onRetry}
              type="button"
            >
              重新加载
            </button>
          </div>
        ) : null}

        {!loading && !error ? (
          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-3">
            {choices.map((choice) => (
              <button
                className={cn(
                  "group flex min-h-16 items-center gap-3 rounded-2xl border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--background)_62%,transparent)] px-3.5 py-3 text-left transition-[border-color,background-color,box-shadow,transform] duration-200",
                  "hover:-translate-y-0.5 hover:border-[color:color-mix(in_srgb,var(--primary)_42%,var(--divider-subtle-color))] hover:bg-[color-mix(in_srgb,var(--primary)_7%,var(--background))] hover:shadow-[0_8px_24px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/35",
                )}
                key={choice.presetKey}
                onClick={() => onSelect(choice)}
                type="button"
              >
                <ProviderIcon
                  name={choice.displayName}
                  presetKey={choice.presetKey}
                  size="md"
                />
                <span className="min-w-0 truncate text-[13px] font-medium text-(--text-strong)">
                  {choice.displayName}
                </span>
              </button>
            ))}
          </div>
        ) : null}

        <div className="mt-4 border-t border-(--divider-subtle-color) pt-4">
          <button
            className={getUiButtonClassName(
              { size: "sm", tone: "default", variant: "ghost" },
              "group px-0 text-(--text-muted)",
            )}
            onClick={() => {
              beginHomeOnboardingProviderConfiguration();
              navigate(AppRouteBuilders.settings("providers"));
            }}
            type="button"
          >
            <Settings2 className="h-4 w-4" />
            其他或自定义模型服务
            <ArrowRight className="h-4 w-4 transition-transform duration-200 group-hover:translate-x-0.5" />
          </button>
        </div>
      </div>
    </section>
  );
}
