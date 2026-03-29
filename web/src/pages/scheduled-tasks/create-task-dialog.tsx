/**
 * 创建定时任务对话框
 *
 * 纯前端占位实现，不需要后端 API 支持。
 * 包含任务名称、执行 Agent、执行频率、执行时间、任务指令等字段。
 */

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

/** 执行频率选项 */
type FrequencyOption = "daily" | "weekly" | "monthly" | "cron";

interface FrequencyDef {
  key: FrequencyOption;
  label: string;
}

const FREQUENCY_OPTIONS: FrequencyDef[] = [
  { key: "daily", label: "每天" },
  { key: "weekly", label: "每周" },
  { key: "monthly", label: "每月" },
  { key: "cron", label: "自定义 Cron" },
];

interface CreateTaskDialogProps {
  is_open: boolean;
  on_close: () => void;
}

export function CreateTaskDialog({ is_open, on_close }: CreateTaskDialogProps) {
  const name_ref = useRef<HTMLInputElement>(null);
  const [task_name, set_task_name] = useState("");
  const [agent, set_agent] = useState("");
  const [frequency, set_frequency] = useState<FrequencyOption>("daily");
  const [time, set_time] = useState("09:00");
  const [instruction, set_instruction] = useState("");

  // 打开时聚焦到名称输入框
  useEffect(() => {
    if (is_open && name_ref.current) {
      name_ref.current.focus();
    }
  }, [is_open]);

  // ESC 关闭
  useEffect(() => {
    const handle_key_down = (e: KeyboardEvent) => {
      if (!is_open) return;
      if (e.key === "Escape") {
        e.preventDefault();
        on_close();
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_open, on_close]);

  // 重置表单
  useEffect(() => {
    if (is_open) {
      set_task_name("");
      set_agent("");
      set_frequency("daily");
      set_time("09:00");
      set_instruction("");
    }
  }, [is_open]);

  if (!is_open) return null;

  /** 提交处理（占位，仅关闭对话框） */
  const handle_submit = () => {
    on_close();
  };

  return (
    <div
      aria-labelledby="create-task-dialog-title"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-150"
      role="dialog"
    >
      <div className="soft-ring radius-shell-lg panel-surface w-full max-w-lg p-6 animate-in zoom-in-95 duration-150">
        {/* 标题栏 */}
        <div className="flex items-start justify-between gap-3 pb-4">
          <h3
            className="text-base font-semibold text-foreground"
            id="create-task-dialog-title"
          >
            创建定时任务
          </h3>
          <button
            aria-label="关闭"
            className="neo-pill radius-shell-sm p-1 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_close}
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* 表单内容 */}
        <div className="flex flex-col gap-4">
          {/* 任务名称 */}
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="task-name">
              任务名称
            </label>
            <input
              ref={name_ref}
              className="neo-inset radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-name"
              onChange={(e) => set_task_name(e.target.value)}
              placeholder="输入任务名称"
              type="text"
              value={task_name}
            />
          </div>

          {/* 执行 Agent */}
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="task-agent">
              执行 Agent
            </label>
            <select
              className="neo-inset radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-agent"
              onChange={(e) => set_agent(e.target.value)}
              value={agent}
            >
              <option value="">选择 Agent</option>
              {/* 占位选项，后续接入真实 Agent 列表 */}
              <option value="default">默认 Agent</option>
            </select>
          </div>

          {/* 执行频率 */}
          <div className="flex flex-col gap-1.5">
            <span className="text-sm font-medium text-foreground">执行频率</span>
            <div className="flex flex-wrap gap-2">
              {FREQUENCY_OPTIONS.map((opt) => (
                <button
                  className={cn(
                    "radius-shell-sm px-3 py-1.5 text-sm font-medium transition-colors",
                    frequency === opt.key
                      ? "bg-primary text-primary-foreground shadow-[0_8px_18px_rgba(133,119,255,0.18)]"
                      : "neo-pill text-muted-foreground hover:text-foreground",
                  )}
                  key={opt.key}
                  onClick={() => set_frequency(opt.key)}
                  type="button"
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          {/* 执行时间 */}
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="task-time">
              执行时间
            </label>
            <input
              className="neo-inset radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-time"
              onChange={(e) => set_time(e.target.value)}
              type="time"
              value={time}
            />
          </div>

          {/* 任务指令 */}
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="task-instruction">
              任务指令
            </label>
            <textarea
              className="neo-inset radius-shell-sm w-full resize-none px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-instruction"
              onChange={(e) => set_instruction(e.target.value)}
              placeholder="输入 Agent 需要执行的指令"
              rows={3}
              value={instruction}
            />
          </div>
        </div>

        {/* 操作按钮 */}
        <div className="mt-5 flex justify-end gap-2">
          <button
            className="neo-pill radius-shell-sm px-4 py-2 text-sm font-medium text-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={on_close}
            type="button"
          >
            取消
          </button>
          <button
            className="radius-shell-sm bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-[0_16px_28px_rgba(133,119,255,0.22)] transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            onClick={handle_submit}
            type="button"
          >
            创建
          </button>
        </div>
      </div>
    </div>
  );
}
