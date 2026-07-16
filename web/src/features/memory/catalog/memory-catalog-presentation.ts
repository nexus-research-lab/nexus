import {
  BookOpenText,
  FileText,
  FolderKanban,
  History,
  Link2,
  MessageSquareWarning,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import type { TranslationKey } from "@/shared/i18n/messages";
import type { MemoryDocument } from "@/types/memory/memory";

interface MemoryDocumentPresentation {
  icon: LucideIcon;
  labelKey: TranslationKey;
  tone: string;
}

type MemoryPresentationKey = "index" | "daily_log" | "user" | "feedback" | "project" | "reference" | "topic";

const PRESENTATION_BY_KEY: Readonly<Record<MemoryPresentationKey, MemoryDocumentPresentation>> = {
  index: {
    icon: BookOpenText,
    labelKey: "capability.memory_index",
    tone: "bg-(--surface-panel-subtle-background) text-(--text-default)",
  },
  daily_log: {
    icon: History,
    labelKey: "capability.memory_type_daily_log",
    tone: "bg-(--surface-panel-subtle-background) text-(--text-muted)",
  },
  user: {
    icon: UserRound,
    labelKey: "capability.memory_type_user",
    tone: "bg-[color:color-mix(in_srgb,var(--accent)_10%,transparent)] text-(--accent)",
  },
  feedback: {
    icon: MessageSquareWarning,
    labelKey: "capability.memory_type_feedback",
    tone: "bg-[color:color-mix(in_srgb,var(--warning)_10%,transparent)] text-(--warning)",
  },
  project: {
    icon: FolderKanban,
    labelKey: "capability.memory_type_project",
    tone: "bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)",
  },
  reference: {
    icon: Link2,
    labelKey: "capability.memory_type_reference",
    tone: "bg-(--surface-panel-subtle-background) text-(--text-muted)",
  },
  topic: {
    icon: FileText,
    labelKey: "capability.memory_type_topic",
    tone: "bg-(--surface-panel-subtle-background) text-(--text-muted)",
  },
};

export function getMemoryDocumentPresentation(
  document: MemoryDocument,
): MemoryDocumentPresentation {
  const key = document.kind === "topic"
    ? document.type || "topic"
    : document.kind;
  return PRESENTATION_BY_KEY[key];
}
