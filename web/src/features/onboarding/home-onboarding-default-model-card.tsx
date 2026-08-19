import { Check, Sparkles } from "lucide-react";

import { getUiButtonClassName } from "@/shared/ui/button/button-styles";
import type { ProviderModelSelection } from "@/types/capability/provider";

interface HomeOnboardingDefaultModelCardProps {
  choices: ProviderModelSelection[];
  onConfirm: () => void;
  onSelectionChange: (selection: ProviderModelSelection) => void;
  selection: ProviderModelSelection;
}

export function HomeOnboardingDefaultModelCard({
  choices,
  onConfirm,
  onSelectionChange,
  selection,
}: HomeOnboardingDefaultModelCardProps) {
  const selectionIndex = Math.max(
    0,
    choices.findIndex(
      (choice) =>
        choice.provider === selection.provider && choice.model === selection.model,
    ),
  );
  return (
    <section
      aria-labelledby="home-onboarding-default-model-card-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_12%,transparent),transparent_58%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <Sparkles className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0">
            <h3
              className="text-[15px] font-semibold text-(--text-strong)"
              id="home-onboarding-default-model-card-title"
            >
              选择默认对话模型
            </h3>
            <p className="mt-1 text-[13px] leading-6 text-(--text-muted)">
              已自动同步该厂商支持的模型。选择后，Nexus Agent
              会使用它完成主页引导和对话任务。
            </p>
          </div>
        </div>
      </div>

      <div className="px-5 py-5 sm:px-6">
        <div className="rounded-2xl border border-(--divider-subtle-color) bg-(--surface-subtle) px-4 py-3">
          <label
            className="text-[12px] text-(--text-muted)"
            htmlFor="home-onboarding-model-select"
          >
            选择对话模型
          </label>
          <select
            className="mt-2 w-full rounded-xl border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2 text-[14px] font-semibold text-(--text-strong) outline-none focus:border-(--primary)"
            id="home-onboarding-model-select"
            onChange={(event) => {
              const nextSelection = choices[Number(event.target.value)];
              if (nextSelection) {
                onSelectionChange(nextSelection);
              }
            }}
            value={String(selectionIndex)}
          >
            {choices.map((choice, index) => (
              <option
                key={`${choice.provider}-${choice.model}`}
                value={String(index)}
              >
                {choice.provider_display_name || choice.provider} /{" "}
                {choice.model_display_name || choice.model}
              </option>
            ))}
          </select>
        </div>
        <button
          className={getUiButtonClassName(
            { size: "md", tone: "primary", variant: "solid" },
            "mt-5 w-full justify-center rounded-xl",
          )}
          onClick={onConfirm}
          type="button"
        >
          <Check className="h-4 w-4" />
          设为默认对话模型
        </button>
      </div>
    </section>
  );
}
