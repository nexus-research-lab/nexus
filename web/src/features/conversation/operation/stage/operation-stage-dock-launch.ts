import type {
  StageWindowKind,
  StageWindowLayout,
  StageWindowPayload,
  StageWindowState,
} from "../operation-desktop-types";
import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationKind,
  OperationSurface,
} from "../operation-types";

export function stage_dock_launch_window_id(
  round_id: string,
  kind: StageWindowKind,
): string {
  return `dock-launch:${round_id}:${kind}`;
}

export function build_stage_dock_launch_window({
  app_label,
  event,
  kind,
  snapshot,
}: {
  app_label: string;
  event: NexusOperationEvent;
  kind: StageWindowKind;
  snapshot: NexusOperationSnapshot | null;
}): StageWindowState {
  const synthetic_event = build_dock_launch_event({ app_label, event, kind });
  const payload = build_dock_launch_payload({ event: synthetic_event, kind, snapshot });

  return {
    id: stage_dock_launch_window_id(event.round_id, kind),
    kind,
    layout: dock_launch_layout_for_kind(kind),
    payload,
    phase: "focused",
    subtitle: "从 Dock 打开",
    target: payload.target ?? synthetic_event.target,
    title: dock_launch_title_for_kind(kind, app_label),
    z: 48,
  };
}

function build_dock_launch_event({
  app_label,
  event,
  kind,
}: {
  app_label: string;
  event: NexusOperationEvent;
  kind: StageWindowKind;
}): NexusOperationEvent {
  return {
    ...event,
    id: stage_dock_launch_window_id(event.round_id, kind),
    evidence: [],
    input_preview: null,
    kind: dock_launch_operation_kind_for_kind(kind),
    phase: "running",
    result_preview: null,
    surface: dock_launch_surface_for_kind(kind),
    target: dock_launch_target_for_kind(kind),
    title: dock_launch_title_for_kind(kind, app_label),
    tool_name: app_label,
    tool_use_id: null,
    updated_at: Date.now(),
  };
}

function build_dock_launch_payload({
  event,
  kind,
  snapshot,
}: {
  event: NexusOperationEvent;
  kind: StageWindowKind;
  snapshot: NexusOperationSnapshot | null;
}): StageWindowPayload {
  const target = dock_launch_target_for_kind(kind);
  const base: StageWindowPayload = {
    event,
    snapshot,
    summary: dock_launch_summary_for_kind(kind),
    target,
  };

  if (kind === "finder") {
    return {
      ...base,
      workspace_items: snapshot?.workspace_events ?? [],
    };
  }
  if (kind === "terminal") {
    return {
      ...base,
      command: "",
      lines: ["$ ", "nexus shell ready", "workspace mounted"],
    };
  }
  if (kind === "browser") {
    return {
      ...base,
      lines: ["Safari 已就绪", "等待 Nexus 打开网页或本地预览。"],
      query: "about:blank",
      url: "about:blank",
    };
  }
  if (kind === "code_editor") {
    return {
      ...base,
      preview: [
        "/* Nexus Code */",
        "const workspace = \"mounted\";",
        "const nextAction = \"waiting for tool event\";",
        "",
        "export function ready() {",
        "  return \"Code is ready\";",
        "}",
      ].join("\n"),
      summary: "Code 已打开，等待 Nexus 创建或修改文件。",
      target,
    };
  }
  if (kind === "image_viewer") {
    return {
      ...base,
      preview: "没有选中的图像。Preview 会在 Nexus 生成图片或打开文件后显示内容。",
      summary: "预览已打开，等待选择文件。",
      target,
    };
  }
  if (kind === "run_manifest" || kind === "handoff") {
    return {
      ...base,
      evidence: snapshot?.recent_evidence ?? [],
      related_events: snapshot?.events ?? [event],
    };
  }
  return {
    ...base,
    preview: "",
  };
}

function dock_launch_layout_for_kind(kind: StageWindowKind): StageWindowLayout {
  if (kind === "finder") {
    return "secondary";
  }
  if (kind === "terminal") {
    return "terminal";
  }
  return "primary";
}

function dock_launch_operation_kind_for_kind(kind: StageWindowKind): OperationKind {
  if (kind === "terminal") {
    return "command_run";
  }
  if (kind === "browser") {
    return "web_research";
  }
  if (kind === "finder") {
    return "workspace_inspect";
  }
  if (kind === "code_editor" || kind === "image_viewer") {
    return "workspace_read";
  }
  if (kind === "handoff") {
    return "round_summary";
  }
  return "plan_update";
}

function dock_launch_surface_for_kind(kind: StageWindowKind): OperationSurface {
  if (kind === "terminal") {
    return "terminal";
  }
  if (kind === "browser") {
    return "web";
  }
  if (kind === "finder") {
    return "workspace";
  }
  if (kind === "code_editor" || kind === "image_viewer") {
    return "editor";
  }
  if (kind === "handoff" || kind === "run_manifest") {
    return "summary";
  }
  return "fallback";
}

function dock_launch_title_for_kind(kind: StageWindowKind, app_label: string): string {
  if (kind === "browser") {
    return "起始页";
  }
  if (kind === "terminal") {
    return "终端";
  }
  if (kind === "code_editor") {
    return "Welcome.tsx";
  }
  if (kind === "image_viewer") {
    return "No Selection.png";
  }
  return app_label;
}

function dock_launch_target_for_kind(kind: StageWindowKind): string {
  if (kind === "browser") {
    return "about:blank";
  }
  if (kind === "terminal") {
    return "shell";
  }
  if (kind === "code_editor") {
    return "Welcome.tsx";
  }
  if (kind === "image_viewer") {
    return "No Selection.png";
  }
  if (kind === "handoff") {
    return "交付台";
  }
  if (kind === "run_manifest") {
    return "控制台";
  }
  return "工作区";
}

function dock_launch_summary_for_kind(kind: StageWindowKind): string {
  if (kind === "browser") {
    return "Safari 已从 Dock 打开，等待 Nexus 加载网页或本地产物。";
  }
  if (kind === "terminal") {
    return "终端已从 Dock 打开，等待运行命令。";
  }
  if (kind === "finder") {
    return "访达已从 Dock 打开，显示当前工作区。";
  }
  if (kind === "handoff") {
    return "交付台已从 Dock 打开，等待本轮产物归档。";
  }
  if (kind === "run_manifest") {
    return "控制台已从 Dock 打开，等待执行记录。";
  }
  return "应用已从 Dock 打开，等待 Nexus 填充内容。";
}
