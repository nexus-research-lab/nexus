import { cn } from "@/shared/ui/class-name";

interface ProviderIconProps {
  active?: boolean;
  name: string;
  presetKey?: string | null;
  size?: "sm" | "md";
}

interface ProviderIconSizeStyle {
  container: string;
  glyph: string;
  initials: string;
}

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

const PROVIDER_ICON_SIZE_STYLE: Record<
  NonNullable<ProviderIconProps["size"]>,
  ProviderIconSizeStyle
> = {
  md: {
    container: "h-10 w-10",
    glyph: "h-6 w-6",
    initials: "text-[13px]",
  },
  sm: {
    container: "h-7 w-7",
    glyph: "h-4.5 w-4.5",
    initials: "text-[9.5px]",
  },
};

const PROVIDER_ICON_BASE_CLASS_NAME =
  "inline-flex shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_82%,white)]";

const PROVIDER_ICON_ACTIVE_CLASS_NAME =
  "border-[color:color-mix(in_srgb,var(--success)_42%,var(--divider-subtle-color))]";

function getCustomProviderInitials(name: string): string {
  const normalized = name.trim() || "AI";
  const words = normalized.split(/[^a-zA-Z0-9]+/).filter(Boolean);
  const firstWord = words[0] ?? normalized;
  if (words.length >= 2) {
    return words.slice(0, 2).map((word) => word[0]).join("").toUpperCase();
  }
  if (/^[A-Z0-9]{2,3}$/.test(firstWord)) {
    return firstWord;
  }
  return firstWord.slice(0, 2).toUpperCase();
}

function ProviderInitialsIcon({
  active,
  name,
  sizeStyle,
}: Required<Pick<ProviderIconProps, "active" | "name">> & {
  sizeStyle: ProviderIconSizeStyle;
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        PROVIDER_ICON_BASE_CLASS_NAME,
        "font-semibold tracking-tight text-(--text-strong)",
        active && PROVIDER_ICON_ACTIVE_CLASS_NAME,
        sizeStyle.container,
        sizeStyle.initials,
      )}
    >
      {getCustomProviderInitials(name)}
    </span>
  );
}

function ProviderMaskIcon({
  active,
  iconSrc,
  sizeStyle,
}: Required<Pick<ProviderIconProps, "active">> & {
  iconSrc: string;
  sizeStyle: ProviderIconSizeStyle;
}) {
  const maskImage = `url(${iconSrc})`;
  return (
    <span
      aria-hidden="true"
      className={cn(
        PROVIDER_ICON_BASE_CLASS_NAME,
        active && PROVIDER_ICON_ACTIVE_CLASS_NAME,
        sizeStyle.container,
      )}
    >
      <span
        className={sizeStyle.glyph}
        style={{
          backgroundColor: "var(--text-strong)",
          maskImage,
          maskPosition: "center",
          maskRepeat: "no-repeat",
          maskSize: "contain",
          WebkitMaskImage: maskImage,
          WebkitMaskPosition: "center",
          WebkitMaskRepeat: "no-repeat",
          WebkitMaskSize: "contain",
        }}
      />
    </span>
  );
}

export function ProviderIcon({
  active = false,
  name,
  presetKey,
  size = "sm",
}: ProviderIconProps) {
  const sizeStyle = PROVIDER_ICON_SIZE_STYLE[size];
  const iconSrc = presetKey ? PROVIDER_ICON_SRC[presetKey] : undefined;
  return iconSrc ? (
    <ProviderMaskIcon
      active={active}
      iconSrc={iconSrc}
      sizeStyle={sizeStyle}
    />
  ) : (
    <ProviderInitialsIcon
      active={active}
      name={name}
      sizeStyle={sizeStyle}
    />
  );
}
