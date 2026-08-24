import type { LucideIcon } from "lucide-react";

export type SidebarPrimaryTab = "chat" | "contacts" | "capabilities";

export interface SidebarPrimaryTabItem {
  anchor: string;
  badgeCount: number;
  icon: LucideIcon;
  key: SidebarPrimaryTab;
  label: string;
}

export interface SidebarPinnedConversationItem {
  active: boolean;
  conversationId: string;
  key: string;
  roomId: string;
  route: string;
  title: string;
}

export interface SidebarUtilityLabels {
  collapse: string;
  expand: string;
  guide: string;
  logout: string;
  settings: string;
}
