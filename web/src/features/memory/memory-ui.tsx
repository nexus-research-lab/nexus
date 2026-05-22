import type { ReactNode } from "react";

import { UiBadge } from "@/shared/ui/badge";
import { memory_status_tone } from "@/features/memory/memory-utils";

export function MemoryStatusBadge({
  status,
}: {
  status: string;
}) {
  return (
    <UiBadge size="xs" tone={memory_status_tone(status)}>
      {status || "未标记"}
    </UiBadge>
  );
}

export function MemoryMetaRow({
  label,
  value,
}: {
  label: string;
  value?: ReactNode;
}) {
  if (!value) {
    return null;
  }
  return (
    <div className="grid min-w-0 grid-cols-[64px_minmax(0,1fr)] gap-2">
      <dt className="truncate font-medium text-(--text-soft)">{label}</dt>
      <dd className="min-w-0 break-words text-(--text-default)">{value}</dd>
    </div>
  );
}
