import { Compass, MoreHorizontal, UsersRound } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";

interface RoomHeaderGuideMenuProps {
  onManageMembers?: () => void;
  onReplayTour?: () => void;
}

export function RoomHeaderGuideMenu({
  onManageMembers,
  onReplayTour,
}: RoomHeaderGuideMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const items = [
    onManageMembers ? {
      icon: <UsersRound className="h-4 w-4 text-(--icon-muted)" />,
      label: t("room.members"),
      value: "members",
    } : null,
    onReplayTour ? {
      icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
      label: t("common.view_guide"),
      value: "guide",
    } : null,
  ].filter((item): item is NonNullable<typeof item> => Boolean(item));

  if (items.length === 0) {
    return null;
  }

  return (
    <>
      <button
        ref={buttonRef}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        className="inline-flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={() => setIsOpen((current) => !current)}
        title={t("common.more_actions")}
        type="button"
      >
        <MoreHorizontal className="h-4 w-4" />
      </button>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("common.more_actions")}
        isOpen={isOpen}
        items={items}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          if (value === "members") {
            onManageMembers?.();
            return;
          }
          onReplayTour?.();
        }}
      />
    </>
  );
}
