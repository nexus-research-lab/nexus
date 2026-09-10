// INPUT: Message avatar source, fallback content and optional detail action.
// OUTPUT: Compact/full message avatar with local image failure recovery.
// POS: Message rail avatar geometry and accessible detail affordance.
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { useState, type MouseEvent, type ReactNode } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";

import { getIconAvatarSrc } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";

type MessageAvatarSize = "full" | "compact";
type MessageAvatarRadius = "content" | "control";

const AVATAR_SIZE_CLASS_MAP: Record<MessageAvatarSize, string> = {
  full: "h-10 w-10",
  compact: "h-6 w-6",
};

const AVATAR_RADIUS_CLASS_MAP: Record<MessageAvatarRadius, string> = {
  content: "surface-radius-md",
  control: "radius-control-sm",
};

export function MessageAvatar({
  ariaLabel,
  avatarUrl,
  children,
  className,
  onClick,
  radius,
  size = "full",
  title,
}: {
  ariaLabel?: string;
  avatarUrl?: string | null;
  children?: ReactNode;
  className?: string;
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
  radius?: MessageAvatarRadius;
  size?: MessageAvatarSize;
  title?: string;
}) {
  const resolvedAvatarUrl = getIconAvatarSrc(avatarUrl);
  const shellClassName = cn(
    "overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
    "transition-[border-color,background-color] duration-(--motion-duration-fast) ease-out",
    AVATAR_SIZE_CLASS_MAP[size],
    AVATAR_RADIUS_CLASS_MAP[radius ?? resolveAvatarRadius(size)],
    className,
  );
  const content = (
    <MessageAvatarContent
      key={resolvedAvatarUrl}
      avatarUrl={resolvedAvatarUrl}
      fallback={children}
    />
  );
  return onClick ? (
    <InteractiveMessageAvatar
      ariaLabel={ariaLabel}
      className={shellClassName}
      onClick={onClick}
      title={title}
    >
      {content}
    </InteractiveMessageAvatar>
  ) : (
    <StaticMessageAvatar
      className={shellClassName}
      hasImage={Boolean(resolvedAvatarUrl)}
      title={title}
    >
      {content}
    </StaticMessageAvatar>
  );
}

function resolveAvatarRadius(size: MessageAvatarSize): MessageAvatarRadius {
  return size === "compact" ? "control" : "content";
}

function MessageAvatarContent({
  avatarUrl,
  fallback,
}: {
  avatarUrl: string | null;
  fallback?: ReactNode;
}) {
  const [failed, setFailed] = useState(false);
  if (!avatarUrl || failed) {
    return (
      <span className="flex h-full w-full items-center justify-center text-(--surface-avatar-foreground)">
        {fallback}
      </span>
    );
  }
  return (
    <img
      alt=""
      className="h-full w-full object-cover"
      src={avatarUrl}
      onError={() => setFailed(true)}
    />
  );
}

function InteractiveMessageAvatar({
  ariaLabel,
  children,
  className,
  onClick,
  title,
}: {
  ariaLabel?: string;
  children: ReactNode;
  className: string;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
  title?: string;
}) {
  const { t } = useI18n();
  return (
    <UiTooltip label={title}><button
      aria-label={ariaLabel ?? t("message.avatar_details")}
      className={cn(
        className,
        "motion-safe:hover:border-(--surface-interactive-active-border) cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/45",
      )}
      onClick={onClick}

      type="button"
    >
      {children}
    </button></UiTooltip>
  );
}

function StaticMessageAvatar({
  children,
  className,
  hasImage,
  title,
}: {
  children: ReactNode;
  className: string;
  hasImage: boolean;
  title?: string;
}) {
  return (
    <UiTooltip label={title}><div
      className={cn(
        className,
        !hasImage && "flex items-center justify-center text-(--surface-avatar-foreground)",
      )}

    >
      {children}
    </div></UiTooltip>
  );
}
