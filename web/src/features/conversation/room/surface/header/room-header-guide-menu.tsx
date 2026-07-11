import { Compass, MoreHorizontal } from "lucide-react";
import { useRef, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiActionMenu } from "@/shared/ui/action-menu";

interface RoomHeaderGuideMenuProps {
  onReplayTour: () => void;
}

export function RoomHeaderGuideMenu({
  onReplayTour,
}: RoomHeaderGuideMenuProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);

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
        items={[{
          icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
          label: t("common.view_guide"),
          value: "guide",
        }]}
        onClose={() => setIsOpen(false)}
        onSelect={onReplayTour}
      />
    </>
  );
}
