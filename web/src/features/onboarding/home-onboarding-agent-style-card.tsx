/**
 * [INPUT]: 当前角色的推荐工作风格与选择命令。
 * [OUTPUT]: 对话流中的有限风格选择卡片。
 * [POS]: 专属 Agent 创建任务的第三步交互视图。
 */

import { Compass, Sparkles } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

import type {
  HomeOnboardingAgentStyleChoice,
} from "./home-onboarding-agent-task";

interface HomeOnboardingAgentStyleCardProps {
  choices: HomeOnboardingAgentStyleChoice[];
  onSelect: (choice: HomeOnboardingAgentStyleChoice) => void;
  role: string;
}

export function HomeOnboardingAgentStyleCard({
  choices,
  onSelect,
  role,
}: HomeOnboardingAgentStyleCardProps) {
  return (
    <section
      aria-labelledby="home-onboarding-agent-style-title"
      className="nexus-onboarding-provider-card mx-auto mb-5 mt-1 w-full max-w-[760px] overflow-hidden rounded-[20px] border border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_18px_52px_color-mix(in_srgb,var(--primary)_14%,transparent)]"
    >
      <div className="border-b border-(--divider-subtle-color) bg-[linear-gradient(135deg,color-mix(in_srgb,var(--primary)_13%,transparent),transparent_60%)] px-5 py-5 sm:px-6">
        <div className="flex items-start gap-3.5">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-[color-mix(in_srgb,var(--primary)_14%,transparent)] text-primary shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_22%,transparent)]">
            <Compass className="h-[18px] w-[18px]" />
          </div>
          <div className="min-w-0">
            <h3
              className="text-[15px] font-semibold text-(--text-strong)"
              id="home-onboarding-agent-style-title"
            >
              选择智能体的工作风格
            </h3>
            <p className="mt-1 text-[13px] leading-6 text-(--text-muted)">
              我会把选择转成 {role || "当前角色"} 智能体的身份标签。
            </p>
          </div>
        </div>
      </div>

      <div className="grid gap-2.5 px-5 py-5 sm:px-6">
        {choices.map((choice) => (
          <button
            className={cn(
              "group flex w-full items-start gap-3.5 rounded-2xl border border-(--divider-subtle-color) bg-[color-mix(in_srgb,var(--background)_62%,transparent)] px-4 py-3.5 text-left",
              "transition-[border-color,background-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[color:color-mix(in_srgb,var(--primary)_42%,var(--divider-subtle-color))] hover:bg-[color-mix(in_srgb,var(--primary)_7%,var(--background))] hover:shadow-[0_8px_24px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/35",
            )}
            key={choice.label}
            onClick={() => onSelect(choice)}
            type="button"
          >
            <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
            <span className="min-w-0">
              <span className="block text-[13px] font-semibold text-(--text-strong)">
                {choice.label}
              </span>
              <span className="mt-1 block text-[12px] leading-5 text-(--text-muted)">
                {choice.description}
              </span>
              <span className="mt-2 flex flex-wrap gap-1.5">
                {choice.tags.map((tag) => (
                  <span
                    className="rounded-full bg-[color-mix(in_srgb,var(--primary)_10%,transparent)] px-2 py-0.5 text-[10px] font-medium text-primary"
                    key={tag}
                  >
                    {tag}
                  </span>
                ))}
              </span>
            </span>
          </button>
        ))}
      </div>
    </section>
  );
}
