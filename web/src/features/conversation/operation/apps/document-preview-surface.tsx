import { useEffect, useMemo, useState } from "react";
import {
  Blocks,
  Braces,
  FilePenLine,
  FilePlus2,
  FileSearch,
  FileSpreadsheet,
  FileText,
  GitBranch,
  ImageIcon,
  Package,
} from "lucide-react";

import { cn } from "@/lib/utils";

import {
  getWorkspaceFilePreviewUrl,
} from "@/lib/api/agent-manage-api";
import {
  basename,
  detect_preview_kind,
  get_preview_lines,
  safe_json_stringify,
} from "../operation-preview";
import { resolve_operation_tool_visual_contract } from "../operation-tool-visual-contract";
import type {
  NexusOperationEvent,
  OperationPhase,
} from "../operation-types";
import { build_code_editor_session_view } from "./code-editor-session";

export function DocumentPreview({
  target,
  summary,
  value,
  fallback_lines,
  diff_stats,
  operation_event,
}: {
  target?: string | null;
  summary?: string | null;
  value: unknown;
  fallback_lines?: string[];
  diff_stats?: { additions: number; deletions: number } | null;
  operation_event?: NexusOperationEvent;
}) {
  const kind = detect_preview_kind(target);
  const raw_lines = get_document_preview_lines(value, kind === "code" || kind === "text" ? 80 : 18, {
    preserve_blank_lines: kind === "code" || kind === "text",
  });
  const lines = raw_lines.length ? raw_lines : (fallback_lines ?? []);
  const display_title = basename(target) || summary || "未命名";

  if (kind === "markdown") {
    return (
      <div className="soft-scrollbar h-full min-h-[240px] overflow-auto bg-white/86 p-5">
        <div className="mb-3 flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) pb-3">
          <div className="min-w-0">
            <p className="truncate text-[13px] font-black tracking-[-0.02em] text-(--text-strong)">{display_title}</p>
            <p className="truncate text-[11px] text-(--text-soft)">{summary ?? "Markdown 文稿"}</p>
          </div>
          {diff_stats ? <DiffStatPill additions={diff_stats.additions} deletions={diff_stats.deletions} /> : null}
        </div>
        <div className="space-y-2.5 text-[12px] leading-5 text-(--text-default)">
          {(lines.length ? lines : ["# Markdown", "等待内容写入..."]).map((line, index) => (
            <MarkdownLine key={`${line}:${index}`} line={line} />
          ))}
        </div>
      </div>
    );
  }

  if (kind === "word" || kind === "pdf") {
    return (
      <div className="flex h-full min-h-[260px] items-start justify-center overflow-auto bg-[#e9eef3] p-5">
        <article className="min-h-full w-full max-w-[420px] rounded-[3px] bg-white px-8 py-7 shadow-[0_20px_52px_rgba(18,28,42,0.16)]">
          <div className="mb-5 flex items-start justify-between gap-4 border-b border-(--divider-subtle-color) pb-4">
            <div className="min-w-0">
              <p className="truncate text-[14px] font-black tracking-[-0.025em] text-(--text-strong)">{display_title}</p>
              <p className="mt-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
                {kind === "word" ? "文稿" : "PDF 页面"}
              </p>
            </div>
            <FileText className="h-4 w-4 shrink-0 text-(--icon-muted)" />
          </div>
          <div className="space-y-3">
            {(lines.length ? lines : [summary ?? "文档预览正在准备", "智能体打开文档后会在这里显示正文结构。"]).slice(0, 8).map((line, index) => (
              <p className={cn(
                "text-[12px] leading-6 text-(--text-default)",
                index === 0 && "text-[16px] font-black tracking-[-0.025em] text-(--text-strong)",
              )} key={`${line}:${index}`}>
                {line}
              </p>
            ))}
          </div>
        </article>
      </div>
    );
  }

  if (kind === "spreadsheet") {
    const sheet_lines = lines.length ? lines : ["name,status,value", "file,updated,1", "tests,passed,3"];
    const rows = sheet_lines.slice(0, 6).map((line) => line.split(/,|\t/).slice(0, 4));
    const column_count = 4;
    return (
      <div className="flex h-full min-h-[240px] flex-col overflow-hidden bg-[#f6f8fb]">
        <div className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-4 py-2.5">
          <div className="min-w-0">
            <p className="truncate text-[12px] font-bold text-(--text-strong)">{display_title}</p>
            <p className="truncate text-[10px] text-(--text-soft)">工作表 1 · {rows.length} 行</p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {diff_stats ? <DiffStatPill additions={diff_stats.additions} deletions={diff_stats.deletions} /> : null}
            <FileSpreadsheet className="h-4 w-4 text-(--icon-muted)" />
          </div>
        </div>
        <div
          className="grid min-w-[420px] flex-1 auto-rows-min overflow-auto text-[11px] text-(--text-default)"
          style={{ gridTemplateColumns: `42px repeat(${column_count}, minmax(84px, 1fr))` }}
        >
          <div className="sticky left-0 top-0 z-20 border-b border-r border-(--divider-subtle-color) bg-[#eef3f8]" />
          {Array.from({ length: column_count }).map((_, column_index) => (
            <div
              className="sticky top-0 z-10 border-b border-r border-(--divider-subtle-color) bg-[#eef3f8] px-2 py-1.5 text-center text-[10px] font-black text-(--text-soft)"
              key={`col:${column_index}`}
            >
              {spreadsheet_column_label(column_index)}
            </div>
          ))}
          {rows.flatMap((row, row_index) => [
            <div
              className="sticky left-0 z-10 border-b border-r border-(--divider-subtle-color) bg-[#eef3f8] px-2 py-2 text-right text-[10px] font-black text-(--text-soft)"
              key={`row:${row_index}`}
            >
              {row_index + 1}
            </div>,
            ...Array.from({ length: column_count }).map((_, column_index) => (
              <div
                className={cn(
                  "min-h-9 truncate border-b border-r border-(--divider-subtle-color) px-2 py-2",
                  row_index === 0 && "bg-[rgba(91,114,255,0.08)] font-bold text-(--text-strong)",
                )}
                key={`${row_index}:${column_index}`}
              >
                {row[column_index] ?? ""}
              </div>
            )),
          ])}
        </div>
        <div className="flex items-center gap-2 border-t border-(--divider-subtle-color) bg-white/62 px-3 py-1.5 text-[10px] text-(--text-soft)">
          <span className="rounded-[7px] border border-[rgba(47,184,132,0.22)] bg-[rgba(47,184,132,0.10)] px-2 py-1 font-bold text-[color:var(--success)]">
            工作表 1
          </span>
          <span>{rows.length} 行 · {column_count} 列</span>
        </div>
      </div>
    );
  }

  if (kind === "image") {
    const image_src = resolve_document_image_src(operation_event, target);
    return (
      <div className="flex h-full min-h-[240px] flex-col overflow-hidden bg-[#eef2f6]">
        <div className="flex items-center justify-between gap-3 border-b border-(--divider-subtle-color) bg-white/62 px-4 py-2.5">
          <div className="min-w-0">
            <p className="truncate text-[12px] font-bold text-(--text-strong)">{display_title}</p>
            <p className="truncate text-[10px] text-(--text-soft)">图像 · {image_format_label(display_title)}</p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {diff_stats ? <DiffStatPill additions={diff_stats.additions} deletions={diff_stats.deletions} /> : null}
            <ImageIcon className="h-4 w-4 text-(--icon-muted)" />
          </div>
        </div>
        <div className="grid min-h-0 flex-1">
          <div className="grid min-h-0 place-items-center overflow-auto p-5">
            <div className="rounded-[14px] border border-white/76 bg-[linear-gradient(45deg,#dfe6ee_25%,transparent_25%,transparent_75%,#dfe6ee_75%),linear-gradient(45deg,#dfe6ee_25%,transparent_25%,transparent_75%,#dfe6ee_75%)] bg-[length:22px_22px] bg-[position:0_0,11px_11px] p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
              {image_src ? (
                <img
                  alt={display_title}
                  className="max-h-[58vh] max-w-full rounded-[10px] border border-white/80 bg-white object-contain shadow-[0_18px_42px_rgba(18,28,42,0.16)]"
                  src={image_src}
                />
              ) : (
                <div className="grid h-32 w-48 place-items-center rounded-[10px] border border-white/80 bg-[radial-gradient(circle_at_34%_30%,rgba(91,114,255,0.26),transparent_30%),linear-gradient(135deg,rgba(47,184,132,0.22),rgba(223,157,46,0.18),rgba(255,255,255,0.72))] shadow-[0_18px_42px_rgba(18,28,42,0.16)]">
                  <ImageIcon className="h-8 w-8 text-white/74 drop-shadow" />
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (kind === "folder") {
    return (
      <div className="soft-scrollbar h-full min-h-[240px] space-y-2 overflow-auto bg-white/86 p-4">
        {(lines.length ? lines : [target ?? "workspace", "src/", "docs/", "package.json"]).slice(0, 9).map((line, index) => (
          <FileRow active={index === 0} key={`${line}:${index}`} label={line} />
        ))}
      </div>
    );
  }

  return (
    <EditorSurface
      diff_stats={diff_stats}
      lines={lines.length ? lines : (fallback_lines ?? [summary ?? "暂无预览"])}
      operation_event={operation_event}
      phase_label={summary ?? "预览"}
      title={display_title}
    />
  );
}

function resolve_document_image_src(
  event: NexusOperationEvent | undefined,
  target?: string | null,
): string | null {
  if (!target) {
    return null;
  }
  if (/^(https?:|data:|blob:)/i.test(target)) {
    return target;
  }
  return event?.agent_id ? getWorkspaceFilePreviewUrl(event.agent_id, target) : null;
}

function EditorSurface({
  diff_stats,
  phase_label,
  title,
  lines,
  operation_event,
}: {
  diff_stats?: { additions: number; deletions: number } | null;
  phase_label: string;
  title: string;
  lines: string[];
  operation_event?: NexusOperationEvent;
}) {
  const session = build_code_editor_session_view({ diff_stats, lines, title });
  const activity = build_editor_activity(operation_event);
  const ActivityIcon = activity.Icon;
  const visual_lines = lines.length ? lines : [""];
  const display_lines = useEditorTypewriterLines({
    enabled: activity.type_lines,
    event_key: operation_event?.id ?? title,
    lines: visual_lines,
  });
  const last_line_index = Math.max(display_lines.length - 1, 0);
  return (
    <div className="flex h-full min-h-[240px] flex-col overflow-hidden bg-[#101820] text-[#dce8ee]">
      <div className="border-b border-white/10 bg-[#151f29]">
        <div className="flex min-w-0 items-end gap-1.5 px-3 pt-1.5">
          <div className="flex min-w-0 max-w-[68%] items-center gap-1.5 rounded-t-[9px] border border-b-0 border-white/10 bg-[#101820] px-3 py-1.5 text-[10px] font-semibold text-[#dce8ee]">
            {session.is_code ? <Braces className="h-3 w-3 shrink-0 text-[#8de0ad]" /> : <FileText className="h-3 w-3 shrink-0 text-[#8aa0ad]" />}
            <span className="truncate">{session.tab_title}</span>
            <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", activity.dot_class)} />
          </div>
          <span className="mb-1 hidden min-w-0 truncate text-[9px] font-semibold text-[#7f94a3] md:block">
            {activity.chrome_label}
          </span>
          <span className="mb-1 ml-auto shrink-0 rounded bg-white/[0.06] px-1.5 py-px text-[9px] font-bold text-[#8aa0ad]">
            {session.extension_label}
          </span>
          {activity.save_label ? (
            <span className={cn(
              "mb-1 shrink-0 rounded px-1.5 py-px text-[9px] font-black",
              activity.save_tone === "saved" ? "bg-[#10271e] text-[#8de0ad]" : "bg-[#2a2412] text-[#ffd27a]",
            )}>
              {activity.save_label}
            </span>
          ) : null}
        </div>
      </div>
      <div className="flex min-h-0 flex-1">
        <div className="hidden w-10 shrink-0 flex-col items-center gap-2 border-r border-white/10 bg-[#0a1118] py-2 text-[#7f94a3] sm:flex">
          <span className="grid h-7 w-7 place-items-center rounded-[8px] bg-white/[0.07] text-[#dce8ee]">
            <FileText className="h-3.5 w-3.5" />
          </span>
          <span className="grid h-7 w-7 place-items-center rounded-[8px] text-[#7f94a3]">
            <GitBranch className="h-3.5 w-3.5" />
          </span>
          <span className="grid h-7 w-7 place-items-center rounded-[8px] text-[#7f94a3]">
            <Blocks className="h-3.5 w-3.5" />
          </span>
        </div>
        <div className="hidden w-[136px] shrink-0 border-r border-white/10 bg-[#0c141c] p-2 text-[10px] text-[#7f94a3] sm:block">
          <div className="mb-2 truncate rounded-md bg-white/[0.06] px-2 py-1.5 font-bold text-[#dce8ee]">资源管理器</div>
          <div className="space-y-1">
            <div className={cn(
              "operation-editor-file-activity flex min-w-0 items-center gap-1.5 truncate rounded px-2 py-1 text-[#dce8ee]",
              activity.sidebar_class,
            )}>
              <ActivityIcon className="h-3 w-3 shrink-0" />
              <span className="truncate">{title}</span>
            </div>
            <div className="flex items-center gap-1.5 truncate rounded px-2 py-1">
              <Package className="h-3 w-3 shrink-0" />
              <span className="truncate">workspace</span>
            </div>
            <div className="truncate rounded px-2 py-1">时间线</div>
          </div>
        </div>
        <div className="soft-scrollbar relative min-w-0 flex-1 overflow-auto p-3 font-mono text-[11px] leading-5">
          {activity.show_scan ? <span className="operation-scan-line" /> : null}
          <div className="mb-2 flex min-w-0 items-center gap-2 border-b border-white/10 pb-2 text-[10px] text-[rgba(220,232,238,0.52)]">
            <span className="truncate">{activity.status_label ?? phase_label}</span>
            {diff_stats ? (
              <span className="shrink-0 rounded bg-[#10271e] px-1.5 py-px text-[#8de0ad]">
                +{diff_stats.additions} -{diff_stats.deletions}
              </span>
            ) : null}
          </div>
          {activity.show_progress ? (
            <div className="mb-2 h-1 overflow-hidden rounded-full bg-white/[0.06]">
              <div className={cn(
                "h-full rounded-full transition-all duration-500",
                activity.progress_done ? "w-full bg-[#8de0ad]" : "w-2/3 bg-[#ffd27a]",
              )} />
            </div>
          ) : null}
          {display_lines.map((line, index) => (
            <div
              className={cn(
                "operation-editor-line flex min-w-0 gap-3 rounded-[4px] px-1",
                activity.active_line && index === last_line_index && "bg-white/[0.045]",
              )}
              key={`${line}:${index}`}
            >
              <span className="w-8 shrink-0 select-none text-right text-[rgba(220,232,238,0.35)]">{index + 1}</span>
              <span className={cn(
                "min-w-0 whitespace-pre-wrap break-words",
                line.startsWith("+") && "text-[#8de0ad]",
                line.startsWith("-") && "text-[#ff9d9d]",
              )}>
                <span>{line || " "}</span>
                {activity.show_caret && index === last_line_index ? (
                  <span className="operation-editor-caret" />
                ) : null}
              </span>
            </div>
          ))}
        </div>
      </div>
      <div className="flex min-w-0 items-center justify-between gap-3 border-t border-white/10 bg-[#0c141c] px-3 py-1.5 text-[10px] text-[#7f94a3]">
        <span className="truncate">{activity.footer_label ?? session.status_label}</span>
        <span className="shrink-0">{session.cursor_label}</span>
      </div>
    </div>
  );
}

function build_editor_activity(event?: NexusOperationEvent): {
  active_line: boolean;
  chrome_label: string;
  dot_class: string;
  footer_label: string | null;
  Icon: typeof FileText;
  progress_done: boolean;
  save_label: string | null;
  save_tone: "saving" | "saved";
  show_caret: boolean;
  show_progress: boolean;
  show_scan: boolean;
  sidebar_class: string;
  status_label: string | null;
  type_lines: boolean;
} {
  if (!event) {
    return {
      active_line: false,
      chrome_label: "预览",
      dot_class: "bg-[#526879]",
      footer_label: null,
      Icon: FileText,
      progress_done: false,
      save_label: null,
      save_tone: "saved",
      show_caret: false,
      show_progress: false,
      show_scan: false,
      sidebar_class: "bg-white/[0.06]",
      status_label: null,
      type_lines: false,
    };
  }

  const visual_contract = resolve_operation_tool_visual_contract(event);
  const is_live = event.phase === "running" || event.phase === "queued" || event.phase === "waiting";
  if (visual_contract.group === "workspace_writer" && visual_contract.action === "create") {
    return {
      active_line: true,
      chrome_label: phase_editor_label(event.phase, "新建文件", "写入文件"),
      dot_class: "bg-[#8de0ad] shadow-[0_0_10px_rgba(141,224,173,0.55)]",
      footer_label: is_live ? "正在写入文件内容" : "文件已写入",
      Icon: FilePlus2,
      progress_done: !is_live,
      save_label: is_live ? "保存中" : "已保存",
      save_tone: is_live ? "saving" : "saved",
      show_caret: true,
      show_progress: true,
      show_scan: false,
      sidebar_class: "bg-[#8de0ad]/10 text-[#dce8ee]",
      status_label: is_live ? "光标正在输入" : "内容已落盘",
      type_lines: true,
    };
  }
  if (visual_contract.group === "workspace_writer") {
    return {
      active_line: true,
      chrome_label: phase_editor_label(event.phase, "编辑文件", "更新文件"),
      dot_class: "bg-[#ffd27a] shadow-[0_0_10px_rgba(255,210,122,0.45)]",
      footer_label: is_live ? "正在更新文件内容" : "修改已写入",
      Icon: FilePenLine,
      progress_done: !is_live,
      save_label: is_live ? "保存中" : "已保存",
      save_tone: is_live ? "saving" : "saved",
      show_caret: true,
      show_progress: true,
      show_scan: false,
      sidebar_class: "bg-[#ffd27a]/10 text-[#dce8ee]",
      status_label: is_live ? "正在修改当前文件" : "修改完成",
      type_lines: true,
    };
  }
  if (visual_contract.group === "workspace_reader") {
    return {
      active_line: false,
      chrome_label: phase_editor_label(event.phase, "打开文件", "读取文件"),
      dot_class: "bg-[#7fb5ff] shadow-[0_0_10px_rgba(127,181,255,0.45)]",
      footer_label: is_live ? "正在读取文件" : "文件已打开",
      Icon: FileSearch,
      progress_done: !is_live,
      save_label: null,
      save_tone: "saved",
      show_caret: false,
      show_progress: false,
      show_scan: is_live,
      sidebar_class: "bg-[#7fb5ff]/10 text-[#dce8ee]",
      status_label: is_live ? "正在扫描文件内容" : "读取完成",
      type_lines: false,
    };
  }

  return {
    active_line: false,
    chrome_label: visual_contract.interaction_label,
    dot_class: "bg-[#526879]",
    footer_label: null,
    Icon: FileText,
    progress_done: false,
    save_label: null,
    save_tone: "saved",
    show_caret: false,
    show_progress: false,
    show_scan: false,
    sidebar_class: "bg-white/[0.06]",
    status_label: null,
    type_lines: false,
  };
}

function get_document_preview_lines(
  value: unknown,
  max_lines: number,
  options?: { preserve_blank_lines?: boolean },
): string[] {
  if (value == null) {
    return [];
  }

  if (!options?.preserve_blank_lines) {
    return get_preview_lines(value, max_lines);
  }

  const text = typeof value === "string"
    ? value
    : safe_json_stringify(value);

  return text
    .split(/\r?\n/)
    .map((line) => line.trimEnd())
    .slice(0, max_lines);
}

function phase_editor_label(phase: OperationPhase, live_label: string, done_label: string): string {
  if (phase === "error" || phase === "cancelled") {
    return "操作中断";
  }
  if (phase === "done") {
    return done_label;
  }
  return live_label;
}

function useEditorTypewriterLines({
  enabled,
  event_key,
  lines,
}: {
  enabled: boolean;
  event_key: string;
  lines: string[];
}): string[] {
  const text = useMemo(() => lines.join("\n"), [lines]);
  const [visible_text, set_visible_text] = useState(enabled ? "" : text);

  useEffect(() => {
    if (!enabled || prefers_reduced_motion()) {
      set_visible_text(text);
      return;
    }

    let cancelled = false;
    let visible_length = 0;
    const chunk_size = Math.max(18, Math.ceil(Math.max(text.length, 1) / 72));
    set_visible_text("");

    const tick = () => {
      if (cancelled) {
        return;
      }
      visible_length = Math.min(text.length, visible_length + chunk_size);
      set_visible_text(text.slice(0, visible_length));
      if (visible_length < text.length) {
        window.setTimeout(tick, 24);
      }
    };

    const timeout = window.setTimeout(tick, 90);
    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [enabled, event_key, text]);

  const rendered_text = enabled ? visible_text : text;
  const rendered_lines = rendered_text.split("\n");
  return rendered_lines.length ? rendered_lines : [""];
}

function prefers_reduced_motion(): boolean {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function MarkdownLine({ line }: { line: string }) {
  const trimmed = line.trim();
  if (trimmed.startsWith("# ")) {
    return <h3 className="text-[18px] font-black tracking-[-0.035em] text-(--text-strong)">{trimmed.slice(2)}</h3>;
  }
  if (trimmed.startsWith("## ")) {
    return <h4 className="pt-1 text-[14px] font-black tracking-[-0.02em] text-(--text-strong)">{trimmed.slice(3)}</h4>;
  }
  if (trimmed.startsWith("- ") || trimmed.startsWith("* ")) {
    return (
      <div className="flex gap-2">
        <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-[color:var(--primary)]" />
        <p>{trimmed.slice(2)}</p>
      </div>
    );
  }
  if (trimmed.startsWith("```")) {
    return <div className="h-px bg-(--divider-subtle-color)" />;
  }
  return <p className="whitespace-pre-wrap break-words">{line || " "}</p>;
}

function DiffStatPill({ additions, deletions }: { additions: number; deletions: number }) {
  return (
    <span className="shrink-0 rounded-full border border-[rgba(47,184,132,0.22)] bg-[rgba(47,184,132,0.10)] px-2 py-1 text-[10px] font-semibold text-[color:var(--success)]">
      +{additions} -{deletions}
    </span>
  );
}

function spreadsheet_column_label(index: number): string {
  return String.fromCharCode("A".charCodeAt(0) + index);
}

function image_format_label(title: string): string {
  const extension = title.includes(".") ? title.slice(title.lastIndexOf(".") + 1).toUpperCase() : "IMAGE";
  return extension || "IMAGE";
}

function FileRow({
  label,
  meta,
  active = false,
}: {
  label: string;
  meta?: string;
  active?: boolean;
}) {
  return (
    <div className={cn(
      "flex items-center gap-2 rounded-[10px] border px-3 py-2 text-[11px]",
      active
        ? "border-[rgba(79,162,159,0.32)] bg-[rgba(79,162,159,0.14)] text-(--text-strong)"
        : "border-(--divider-subtle-color) bg-white/62 text-(--text-muted)",
    )}>
      <FileText className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {meta ? <span className="shrink-0 font-mono text-[10px] text-(--text-soft)">{meta}</span> : null}
    </div>
  );
}
