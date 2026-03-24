import { PointerEvent as ReactPointerEvent, ReactNode, RefObject } from "react";

import { BlobDebugTarget } from "@/features/launcher-search/launcher-blob-debug-hooks";
import { BlobPoint } from "@/features/launcher-search/launcher-blob-shape";
import { Agent } from "@/types/agent";
import { Conversation } from "@/types/conversation";
import { ConversationWithOwner, SpotlightToken } from "@/types/launcher";

export interface GlassGradientStop {
  color: string;
  offset: string;
}

export interface HeroBlobShellProps {
  children: ReactNode;
  class_name?: string;
}

export interface HeroInputShellProps {
  children: ReactNode;
  class_name?: string;
}

export interface StaticGlassShellProps {
  aura_background?: string;
  aura_blur_class_name?: string;
  children: ReactNode;
  class_name?: string;
  content_class_name?: string;
  fill: string;
  fill_gradient_stops?: GlassGradientStop[];
  glow_blur_deviation?: number;
  inner_fill: string;
  inner_fill_gradient_stops?: GlassGradientStop[];
  inner_glow_opacity?: number;
  inner_path: string;
  inner_stroke: string;
  outer_glow_opacity?: number;
  outer_glow_width?: number;
  path: string;
  stroke: string;
  svg_overlay?: ReactNode;
  view_box_height: number;
  view_box_width: number;
}

export interface HeroActionPillShellProps {
  is_active?: boolean;
  children: ReactNode;
  class_name?: string;
}

export interface HeroActionOrbShellProps {
  is_active?: boolean;
  children: ReactNode;
  class_name?: string;
}

export interface LauncherConsoleProps {
  agents: Agent[];
  conversations: Conversation[];
  current_agent_id: string | null;
  on_open_contacts_page: () => void;
  on_open_nexus: () => void;
  on_select_agent: (agent_id: string) => void;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  on_create_agent: () => void;
  on_edit_agent: (agent_id: string) => void;
  on_delete_agent: (agent_id: string) => void;
}

export interface HeaderActionButtonProps {
  is_active?: boolean;
  children: string;
  on_click: () => void;
}

export interface HeroStageProps {
  current_agent_id: string | null;
  decorative_tokens: SpotlightToken[];
  on_open_nexus: () => void;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  on_query_change: (value: string) => void;
  on_select_agent: (agent_id: string) => void;
  on_submit: () => void;
  query: string;
  recent_agents: Agent[];
  recent_rooms: ConversationWithOwner[];
}

export interface ContactsPopoverProps {
  agents: Agent[];
  on_close: () => void;
  on_create_agent: () => void;
  on_delete_agent: (agent_id: string) => void;
  on_edit_agent: (agent_id: string) => void;
  on_open_contacts_page: () => void;
  on_select_agent: (agent_id: string) => void;
}

export interface RecentRoomsPopoverProps {
  on_close: () => void;
  on_open_conversation: (conversation_id: string, agent_id?: string) => void;
  recent_rooms: ConversationWithOwner[];
  conversations_with_owners: ConversationWithOwner[];
}

export interface BlobDebugPanelProps {
  count_label?: string;
  current_target: BlobDebugTarget;
  description?: string;
  on_copy: () => void;
  on_reset: () => void;
  panel_class_name: string;
  points: BlobPoint[];
  set_target: (target: BlobDebugTarget) => void;
  target: BlobDebugTarget;
  title: string;
}

export interface BlobDebugOverlayProps {
  color: string;
  debug_enabled: boolean;
  fill: string;
  on_path_double_click: (event: ReactPointerEvent<SVGPathElement>) => void;
  on_point_pointer_down: (index: number) => (event: ReactPointerEvent<Element>) => void;
  on_point_pointer_up: (event: ReactPointerEvent<Element>) => void;
  path: string;
  points: BlobPoint[];
  stroke: string;
  stroke_width: number;
  svg_ref: RefObject<SVGSVGElement | null>;
  view_box_height: number;
  view_box_width: number;
}

export interface BlobDebugControllerProps extends BlobDebugOverlayProps {
  is_active: boolean;
  current_target: BlobDebugTarget;
  on_copy: () => void;
  on_reset: () => void;
  panel_class_name: string;
  set_target: (target: BlobDebugTarget) => void;
  show_panel?: boolean;
  target: BlobDebugTarget;
  title: string;
}
