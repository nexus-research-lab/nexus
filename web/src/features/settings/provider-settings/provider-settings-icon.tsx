import { cn } from "@/lib/utils";

const PROVIDER_ICON_SRC: Record<string, string> = {
  anthropic: "/icon/provider/anthropic.svg",
  deepseek: "/icon/provider/deepseek.svg",
  "glm-coding-plan": "/icon/provider/zai.svg",
  "kimi-code": "/icon/provider/moonshot.svg",
  "minimax-token-plan": "/icon/provider/minimax.svg",
  openai: "/icon/provider/openai.svg",
  "qwen-token-plan": "/icon/provider/qwen.svg",
  "volcengine-coding-plan": "/icon/provider/volcengine.svg",
  doubao: "/icon/provider/doubao.svg",
  dashscope: "/icon/provider/alibabacloud.svg",
  modelscope: "/icon/provider/modelscope.svg",
  azure: "/icon/provider/azureai.svg",
};

function get_provider_icon_src(preset_key?: string | null): string {
  return PROVIDER_ICON_SRC[preset_key || ""] ?? "";
}

function get_custom_provider_initials(name: string): string {
  const normalized = name.trim() || "AI";
  const words = normalized.split(/[^a-zA-Z0-9]+/).filter(Boolean);
  const first_word = words[0] ?? normalized;
  if (words.length >= 2) {
    return words.slice(0, 2).map((word) => word[0]).join("").toUpperCase();
  }
  if (/^[A-Z0-9]{2,3}$/.test(first_word)) {
    return first_word;
  }
  return first_word.slice(0, 2).toUpperCase();
}

export function ProviderIcon({
  active = false,
  name,
  preset_key,
  size = "sm",
}: {
  active?: boolean;
  name: string;
  preset_key?: string | null;
  size?: "sm" | "md";
}) {
  if ((preset_key || "custom") === "custom") {
    const initials = get_custom_provider_initials(name);
    return (
      <span
        aria-hidden="true"
        className={cn(
          "inline-flex shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_82%,white)] font-semibold tracking-tight text-(--text-strong)",
          active && "border-[color:color-mix(in_srgb,var(--success)_42%,var(--divider-subtle-color))]",
          size === "md" ? "h-10 w-10 text-[13px]" : "h-7 w-7 text-[9.5px]",
        )}
      >
        {initials}
      </span>
    );
  }

  const icon_src = get_provider_icon_src(preset_key);
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_82%,white)]",
        active && "border-[color:color-mix(in_srgb,var(--success)_42%,var(--divider-subtle-color))]",
        size === "md" ? "h-10 w-10" : "h-7 w-7",
      )}
    >
      <span
        className={cn(size === "md" ? "h-6 w-6" : "h-4.5 w-4.5")}
        style={{
          backgroundColor: "var(--text-strong)",
          maskImage: icon_src ? `url(${icon_src})` : undefined,
          maskPosition: "center",
          maskRepeat: "no-repeat",
          maskSize: "contain",
          WebkitMaskImage: icon_src ? `url(${icon_src})` : undefined,
          WebkitMaskPosition: "center",
          WebkitMaskRepeat: "no-repeat",
          WebkitMaskSize: "contain",
        }}
      />
    </span>
  );
}
