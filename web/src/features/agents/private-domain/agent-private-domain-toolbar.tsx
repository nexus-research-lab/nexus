// INPUT: Private record title.
// OUTPUT: Header title row for the private-domain directory.
// POS: Private-domain header presentation; records refresh via realtime pushes, so no manual control.

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function PrivateDomainToolbar({ title }: { title: string }) {
  return (
    <div className="flex min-h-[48px] items-center gap-3 px-3">
      <span className={cn("min-w-0 truncate", getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "semibold" }))}>{title}</span>
    </div>
  );
}
