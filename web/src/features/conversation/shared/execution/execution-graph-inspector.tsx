// INPUT: Exact selected graph identity, domain heading/content, canvas placement and close command.
// OUTPUT: One screen-sized inspector shell using shared popover, typography and close control recipes.
// POS: Execution-only node/edge detail pattern; the canvas owns selection, panning and inverse zoom geometry.

import type { CSSProperties, ReactNode } from "react";
import { X } from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function ExecutionGraphInspector({
  children, closeLabel, description, detailId, detailKind, heading, label, leading, onClose, style,
}: {
  children: ReactNode;
  closeLabel: string;
  description: ReactNode;
  detailId: string;
  detailKind: "node" | "edge";
  heading: string;
  label: string;
  leading: ReactNode;
  onClose: () => void;
  style: CSSProperties;
}) {
  return (
    <aside
      aria-label={label}
      className="surface-popover soft-scrollbar absolute z-30 max-h-[min(70vh,28rem)] w-[19rem] max-w-[calc(100%-1rem)] cursor-auto overflow-auto"
      data-execution-selected-node-detail={detailKind === "node" ? detailId : undefined}
      data-execution-selected-node-detail-mode={detailKind === "node" ? "popover" : undefined}
      data-execution-selected-edge-detail={detailKind === "edge" ? detailId : undefined}
      style={style}
    >
      <header className="sticky top-0 z-10 flex min-w-0 items-center gap-2 border-b dialog-divider bg-(--surface-popover-background) p-3">
        {leading}
        <div className="min-w-0 flex-1">
          <h3 className={cn("truncate", getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "semibold" }))}>
            {heading}
          </h3>
          <div className={cn("mt-0.5 flex min-w-0 items-center gap-1", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>
            {description}
          </div>
        </div>
        <UiIconButton aria-label={closeLabel} onClick={onClose} size="sm" tooltip={closeLabel} variant="ghost">
          <X aria-hidden="true" className="h-3.5 w-3.5" />
        </UiIconButton>
      </header>
      <div className="space-y-3 p-3">{children}</div>
    </aside>
  );
}
