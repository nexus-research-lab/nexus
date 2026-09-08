// INPUT: User 消息时间、引导状态与可用的重跑/编辑/复制命令。
// OUTPUT: 无气泡消息尾部公共元信息排版与共享微型图标动作。
// POS: User 消息头纯视图；不拥有复制状态或消息 mutation。
import {
  Check,
  Copy,
  CornerDownRight,
  Edit2,
  RotateCcw,
  type LucideIcon,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type { UserMessagePresentation } from "./user-message-model";

interface UserMessageHeaderProps {
  className?: string;
  copied: boolean;
  onCopy: () => Promise<void>;
  onEdit?: () => void;
  onRerun?: () => void;
  presentation: UserMessagePresentation;
}

interface CopyActionPresentation {
  icon: LucideIcon;
  tone: "default" | "success";
}

const COPY_ACTION_PRESENTATION: Record<"copied" | "idle", CopyActionPresentation> = {
  copied: { icon: Check, tone: "success" },
  idle: { icon: Copy, tone: "default" },
};

export function UserMessageHeader({
  className,
  copied,
  onCopy,
  onEdit,
  onRerun,
  presentation,
}: UserMessageHeaderProps) {
  const { t } = useI18n();
  return (
    <div
      className={cn(
        "nexus-chat-user-actions mt-1.5 flex items-center justify-end gap-1 text-(--text-muted) transition-[opacity,transform] duration-(--motion-duration-fast)",
        "sm:pointer-events-none sm:translate-y-0.5 sm:opacity-0 sm:group-focus-within:pointer-events-auto sm:group-focus-within:translate-y-0 sm:group-focus-within:opacity-100 sm:group-hover:pointer-events-auto sm:group-hover:translate-y-0 sm:group-hover:opacity-100",
        className,
      )}
    >
      {presentation.guided ? (
        <span className={cn("mr-1 inline-flex shrink-0 items-center gap-1", getUiTypographyClassName({ role: "caption", tone: "muted", weight: "semibold" }))}>
          <CornerDownRight className="h-3.5 w-3.5" />
          {t("message.guidance")}
        </span>
      ) : null}
      <span className={cn("nexus-chat-meta shrink-0", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
        {presentation.timestamp}
      </span>
      <UserMessageActions
        copied={copied}
        onCopy={onCopy}
        onEdit={onEdit}
        onRerun={onRerun}
      />
    </div>
  );
}

function UserMessageActions({
  copied,
  onCopy,
  onEdit,
  onRerun,
}: Pick<UserMessageHeaderProps, "copied" | "onCopy" | "onEdit" | "onRerun">) {
  const { t } = useI18n();
  const action = COPY_ACTION_PRESENTATION[copied ? "copied" : "idle"];
  const CopyIcon = action.icon;
  return (
    <div className="flex shrink-0 items-center gap-0.5">
      {onRerun ? (
        <UiIconButton
          aria-label={t("message.rerun")}
          onClick={onRerun}
          size="xs"
          tone="default"
          tooltip={t("message.rerun")}
          variant="ghost"
        >
          <RotateCcw className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
      {onEdit ? (
        <UiIconButton
          aria-label={t("message.edit")}
          onClick={onEdit}
          size="xs"
          tone="default"
          tooltip={t("message.edit")}
          variant="ghost"
        >
          <Edit2 className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
      <UiIconButton
        aria-label={t("message.copy")}
        onClick={onCopy}
        size="xs"
        tone={action.tone}
        tooltip={t("message.copy")}
        variant="ghost"
      >
        <CopyIcon className="h-3.5 w-3.5" />
      </UiIconButton>
    </div>
  );
}
