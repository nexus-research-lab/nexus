import {
  Brain,
  CalendarClock,
  Link2,
  MessageCircle,
  type LucideIcon,
  Puzzle,
  Users2,
} from "lucide-react";

import type { WorkspaceFileEntry } from "@/types/agent/agent";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message";
import type { TodoItem } from "@/types/conversation/todo";

export interface LandingCapabilityItem {
  copy: string;
  Icon: LucideIcon;
  meta: string;
  title: string;
}

export const capability_items: LandingCapabilityItem[] = [
  {
    title: "Rooms",
    copy: "Shared context for multi-agent work.",
    meta: "@mentions · directed replies",
    Icon: MessageCircle,
  },
  {
    title: "Agents",
    copy: "Independent identity, workspace, skills, and memory.",
    meta: "DM · profiles · status",
    Icon: Users2,
  },
  {
    title: "Skills",
    copy: "Reusable methods and workflows agents can call.",
    meta: "library · import · update",
    Icon: Puzzle,
  },
  {
    title: "Connectors",
    copy: "Secure access to documents, code, tasks, and IM.",
    meta: "GitHub · Gmail · Slack",
    Icon: Link2,
  },
  {
    title: "Scheduled",
    copy: "Recurring work with sessions, delivery, and history.",
    meta: "one-off · recurring · manual",
    Icon: CalendarClock,
  },
  {
    title: "Memory",
    copy: "Long-term context that can be reviewed and curated.",
    meta: "candidate · promoted · ignored",
    Icon: Brain,
  },
];

export const flow_items = [
  ["01", "Launch", "Start from the launcher. Mention an agent, target a room, or hand the task to Nexus."],
  ["02", "Route", "Nexus opens the right DM or room and keeps the session boundary explicit."],
  ["03", "Run", "Agents use skills, connectors, workspace files, and scheduled runs without leaving the task."],
  ["04", "Review", "Files, history, permissions, and task status stay visible for human review."],
] as const;

export const ledger_items = [
  ["09:42", "Launcher", "Route landing page request"],
  ["09:44", "Workspace", "Read product routes"],
  ["09:51", "Agent", "Update landing-page.tsx"],
  ["10:03", "Control", "Run typecheck, lint, visual QA"],
] as const;

export const hero_route_nodes = [
  ["Launch", "Task enters Nexus"],
  ["Route", "DM or room context"],
  ["Run", "Skills and connectors"],
  ["Review", "Files, history, approval"],
] as const;

export const hero_activity_items = [
  ["10:03", "Nexus", "Split public landing from authenticated app."],
  ["10:05", "Amy", "Reads workspace files and prepares the UI pass."],
  ["10:07", "Control", "Keeps route, history, and validation visible."],
] as const;

export const hero_context_items = ["Rooms", "DMs", "Skills", "Connectors", "Memory", "Schedules", "Files"] as const;

export const demo_conversations: RoomConversationView[] = [
  {
    session_key: "landing-session",
    agent_id: "agent-amy",
    session_id: "sess_landing_demo",
    room_session_id: "room_sess_landing",
    room_id: "room-landing",
    conversation_id: "conv-landing",
    conversation_type: "dm",
    title: "Landing review",
    options: {},
    created_at: 1_746_720_000_000,
    last_activity_at: 1_746_722_400_000,
    is_active: true,
    message_count: 8,
  },
  {
    session_key: "runtime-session",
    agent_id: "agent-amy",
    session_id: "sess_runtime_demo",
    room_session_id: "room_sess_runtime",
    room_id: "room-landing",
    conversation_id: "conv-runtime",
    conversation_type: "dm",
    title: "Runtime QA",
    options: {},
    created_at: 1_746_633_600_000,
    last_activity_at: 1_746_637_200_000,
    is_active: false,
    message_count: 5,
  },
];

export const demo_todos: TodoItem[] = [
  { content: "Read real workspace source", status: "completed" },
  { content: "Remove decorative SVG motion", status: "completed" },
  { content: "Verify route and visual layout", status: "in_progress" },
];

export const demo_workspace_entries: WorkspaceFileEntry[] = [
  { path: "web", name: "web", is_dir: true, modified_at: "2026-06-01T09:42:00Z", depth: 0 },
  { path: "web/src", name: "src", is_dir: true, modified_at: "2026-06-01T09:44:00Z", depth: 1 },
  { path: "web/src/pages", name: "pages", is_dir: true, modified_at: "2026-06-01T09:45:00Z", depth: 2 },
  { path: "web/src/pages/landing", name: "landing", is_dir: true, modified_at: "2026-06-01T09:46:00Z", depth: 3 },
  {
    path: "web/src/pages/landing/landing-page.tsx",
    name: "landing-page.tsx",
    is_dir: false,
    size: 18_420,
    modified_at: "2026-06-01T09:51:00Z",
    depth: 4,
  },
  {
    path: "web/src/pages/landing/landing-page.css",
    name: "landing-page.css",
    is_dir: false,
    size: 13_800,
    modified_at: "2026-06-01T09:52:00Z",
    depth: 4,
  },
  { path: "web/src/app", name: "app", is_dir: true, modified_at: "2026-06-01T09:48:00Z", depth: 2 },
  {
    path: "web/src/app/router/app-router.tsx",
    name: "app-router.tsx",
    is_dir: false,
    size: 4_180,
    modified_at: "2026-06-01T09:49:00Z",
    depth: 3,
  },
  {
    path: "CHANGELOG.md",
    name: "CHANGELOG.md",
    is_dir: false,
    size: 9_280,
    modified_at: "2026-06-01T10:03:00Z",
    depth: 0,
  },
];

export const demo_rounds: Array<{ round_id: string; messages: Message[] }> = [
  {
    round_id: "landing-round-1",
    messages: [
      {
        message_id: "landing-user-1",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-1",
        role: "user",
        timestamp: 1_746_722_180_000,
        content: "Ship the public landing route and keep the review files next to the conversation.",
      },
      {
        message_id: "landing-assistant-1",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-1",
        role: "assistant",
        timestamp: 1_746_722_186_000,
        model: "kimi-for-coding",
        is_complete: true,
        stop_reason: "end_turn",
        content: [
          {
            type: "text",
            text: "I found the route split: public `/`, authenticated `/launcher`, and guarded app surfaces under `/app` and `/rooms`.",
          },
          {
            type: "tool_use",
            id: "tool-read-router",
            name: "Read",
            input: { file_path: "web/src/app/router/app-router.tsx" },
          },
          {
            type: "tool_result",
            tool_use_id: "tool-read-router",
            content: "LandingPage is mounted before AuthGuard. Launcher remains guarded.",
          },
        ],
        result_summary: {
          subtype: "success",
          duration_ms: 3400,
          duration_api_ms: 2900,
          num_turns: 1,
          is_error: false,
          usage: {
            input_tokens: 3620,
            output_tokens: 840,
            cache_read_input_tokens: 15_200,
          },
        },
      },
    ],
  },
  {
    round_id: "landing-round-2",
    messages: [
      {
        message_id: "landing-user-2",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-2",
        role: "user",
        timestamp: 1_746_722_520_000,
        content: "Review the workspace changes and prepare the validation pass.",
      },
      {
        message_id: "landing-assistant-2",
        session_key: "landing-session",
        room_id: "room-landing",
        conversation_id: "conv-landing",
        agent_id: "agent-amy",
        round_id: "landing-round-2",
        role: "assistant",
        timestamp: 1_746_722_529_000,
        model: "kimi-for-coding",
        is_complete: true,
        stop_reason: "end_turn",
        content: [
          {
            type: "text",
            text: "The route, workbench surface, and validation notes are ready for review.",
          },
          {
            type: "workspace_file_artifact",
            path: "web/src/pages/landing/landing-page.tsx",
            display_path: "web/src/pages/landing/landing-page.tsx",
            artifact_kind: "code",
            operation: "updated",
            scope: "agent_workspace",
            workspace_agent_id: "agent-amy",
          },
        ],
        result_summary: {
          subtype: "success",
          duration_ms: 2700,
          duration_api_ms: 2400,
          num_turns: 1,
          is_error: false,
          usage: {
            input_tokens: 4180,
            output_tokens: 690,
            cache_read_input_tokens: 18_900,
          },
        },
      },
    ],
  },
];
