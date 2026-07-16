import type { MouseEvent, ReactNode } from "react";

import { getIconAvatarSrc } from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";

type MessageAvatarSize = "full" | "compact";

const AVATAR_SIZE_CLASS_MAP: Record<MessageAvatarSize, string> = {
  full: "h-10 w-10 rounded-xl",
  compact: "h-6 w-6 rounded-lg",
};

export function MessageAvatar({
  ariaLabel,
  avatarUrl,
  children,
  className,
  onClick,
  size = "full",
  title,
}: {
  ariaLabel?: string;
  avatarUrl?: string | null;
  children?: ReactNode;
  className?: string;
  onClick?: (event: MouseEvent<HTMLButtonElement>) => void;
  size?: MessageAvatarSize;
  title?: string;
}) {
  const resolvedAvatarUrl = getIconAvatarSrc(avatarUrl);
  const shellClassName = cn(
    "overflow-hidden border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
    "transition-[border-color,background-color] duration-(--motion-duration-fast) ease-out",
    "motion-safe:hover:border-(--surface-interactive-active-border)",
    AVATAR_SIZE_CLASS_MAP[size],
    className,
  );
  const content = (
    <MessageAvatarContent
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

function MessageAvatarContent({
  avatarUrl,
  fallback,
}: {
  avatarUrl: string | null;
  fallback?: ReactNode;
}) {
  if (!avatarUrl) {
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
  return (
    <button
      aria-label={ariaLabel ?? "查看头像详情"}
      className={cn(
        className,
        "cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/45",
      )}
      onClick={onClick}
      title={title}
      type="button"
    >
      {children}
    </button>
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
    <div
      className={cn(
        className,
        !hasImage && "flex items-center justify-center text-(--surface-avatar-foreground)",
      )}
      title={title}
    >
      {children}
    </div>
  );
}
