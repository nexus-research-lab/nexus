import { Download } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

export function SidebarUpdateIndicator({
  className,
  version,
}: {
  className?: string;
  version: string;
}) {
  const label = `Nexus ${version} 可更新`;
  return (
    <a
      aria-label={label}
      className={cn("sidebar-update-indicator relative", className)}
      href="https://github.com/nexus-research-lab/nexus/releases/latest"
      rel="noreferrer"
      target="_blank"
      title={label}
    >
      <Download className="h-[18px] w-[18px]" />
      <span className="absolute right-0 top-0 h-2 w-2 rounded-full border-2 border-(--surface-shell-directory-background) bg-(--primary)" />
    </a>
  );
}
