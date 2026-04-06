/**
 * 创建定时任务对话框
 *
 * 纯前端占位实现，不需要后端 API 支持。
 * 包含任务名称、执行 Agent、执行频率、执行时间、任务指令等字段。
 */

import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";

import {
  getDialogChoiceClassName,
  getDialogChoiceStyle,
} from "@/shared/ui/dialog/dialog-styles";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";

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
      className="dialog-backdrop animate-in fade-in duration-150"
      role="dialog"
    >
      <div className="dialog-shell radius-shell-lg w-full max-w-lg animate-in zoom-in-95 duration-150">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <h3 className="dialog-title" id="create-task-dialog-title">
              创建定时任务
            </h3>
            <p className="dialog-subtitle">
              先定义频率和时间，再把执行指令交给 Agent。
            </p>
          </div>
          <WorkspacePillButton
            aria-label="关闭"
            density="compact"
            onClick={on_close}
            size="icon"
            variant="icon"
          >
            <X className="h-4 w-4" />
          </WorkspacePillButton>
        </div>

        <div className="dialog-body flex flex-col gap-4">
          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-name">
              任务名称
            </label>
            <input
              ref={name_ref}
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-name"
              onChange={(e) => set_task_name(e.target.value)}
              placeholder="输入任务名称"
              type="text"
              value={task_name}
            />
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-agent">
              执行 Agent
            </label>
            <select
              className="dialog-input radius-shell-sm w-full appearance-none px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-agent"
              onChange={(e) => set_agent(e.target.value)}
              value={agent}
            >
              <option value="">选择 Agent</option>
              {/* 占位选项，后续接入真实 Agent 列表 */}
              <option value="default">默认 Agent</option>
            </select>
          </div>

          <div className="dialog-field">
            <span className="dialog-label">执行频率</span>
            <div className="flex flex-wrap gap-2">
              {FREQUENCY_OPTIONS.map((opt) => (
                <button
                  className={getDialogChoiceClassName(frequency === opt.key)}
                  key={opt.key}
                  onClick={() => set_frequency(opt.key)}
                  style={getDialogChoiceStyle(frequency === opt.key)}
                  type="button"
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-time">
              执行时间
            </label>
            <input
              className="dialog-input radius-shell-sm w-full px-4 py-2.5 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-time"
              onChange={(e) => set_time(e.target.value)}
              type="time"
              value={time}
            />
          </div>

          <div className="dialog-field">
            <label className="dialog-label" htmlFor="task-instruction">
              任务指令
            </label>
            <textarea
              className="dialog-input radius-shell-sm w-full resize-none px-4 py-2.5 text-sm text-foreground placeholder:text-muted-foreground/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20"
              id="task-instruction"
              onChange={(e) => set_instruction(e.target.value)}
              placeholder="输入 Agent 需要执行的指令"
              rows={3}
              value={instruction}
            />
          </div>
        </div>

        <div className="dialog-footer">
          <WorkspacePillButton onClick={on_close} size="md" variant="tonal">
            取消
          </WorkspacePillButton>
          <WorkspacePillButton onClick={handle_submit} size="md" variant="primary">
            创建
          </WorkspacePillButton>
        </div>
      </div>
    </div>
  );
}
