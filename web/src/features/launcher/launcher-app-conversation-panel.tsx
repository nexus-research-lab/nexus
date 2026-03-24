"use client";

import { useEffect, useMemo, useState } from "react";
import { ArrowRight, Sparkles, Users, X } from "lucide-react";

import {
  HeroActionOrbShell,
  HeroActionPillShell,
  HeroInputShell,
  HeroSidePanelShell,
} from "@/features/launcher/launcher-glass-shell";
import { cn } from "@/lib/utils";

interface AppConversationMessage {
  body: string;
  role: "app" | "user";
}

interface LauncherAppConversationPanelProps {
  app_conversation_draft: string;
  on_change_draft: (next_value: string) => void;
  on_close: () => void;
  on_open_contacts_page: () => void;
  on_submit: (next_prompt: string) => void;
}

function buildConversationMessages(app_conversation_draft: string): AppConversationMessage[] {
  if (!app_conversation_draft.trim()) {
    return [
      {
        role: "app",
        body: "告诉我你想组织什么。我会帮你恢复已有协作、创建 room，或者把合适的成员拉进来。",
      },
    ];
  }

  return [
    {
      role: "user",
      body: app_conversation_draft,
    },
    {
      role: "app",
      body: "我已经收到这条系统级意图。下一步我会围绕这件事组织成员、整理上下文，并把你带到真正承载协作的 room。",
    },
  ];
}

export function LauncherAppConversationPanel({
  app_conversation_draft,
  on_change_draft,
  on_close,
  on_open_contacts_page,
  on_submit,
}: LauncherAppConversationPanelProps) {
  const [draft, set_draft] = useState(app_conversation_draft);

  useEffect(() => {
    set_draft(app_conversation_draft);
  }, [app_conversation_draft]);

  const messages = useMemo(
    () => buildConversationMessages(app_conversation_draft),
    [app_conversation_draft],
  );

  return (
    <HeroSidePanelShell class_name="h-full min-h-[620px] w-full max-w-[380px]">
      <div className="flex h-full flex-col">
        <div className="flex items-start justify-between gap-3">
          <div>
            <HeroActionPillShell class_name="w-fit">
              <span className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-800/72">
                <span className="h-2 w-2 rounded-full bg-[#7fe3a8]" />
                App Agent
              </span>
            </HeroActionPillShell>
            <h2 className="mt-4 text-[28px] font-black tracking-[-0.04em] text-slate-950/88">
              真格 App
            </h2>
            <p className="mt-2 text-sm leading-6 text-slate-700/62">
              这是首页里唯一的全局系统级对话。它负责组织协作，而不是替代 room 承载具体执行。
            </p>
          </div>

          <button
            aria-label="关闭 App 对话"
            className="shrink-0 transition-transform duration-300 hover:-translate-y-0.5"
            onClick={on_close}
            type="button"
          >
            <HeroActionOrbShell class_name="h-[54px] w-[54px]">
              <X className="h-4 w-4 text-slate-900/76" />
            </HeroActionOrbShell>
          </button>
        </div>

        <div className="mt-6 grid grid-cols-2 gap-3">
          <button
            className="rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
            onClick={() => on_submit("帮我恢复最近的协作 room")}
            type="button"
          >
            <p className="text-sm font-semibold text-slate-950/84">恢复最近协作</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              从最近的 room 和对话里继续，而不是重新开始。
            </p>
          </button>

          <button
            className="rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
            onClick={() => on_submit("帮我创建一个新的协作 room")}
            type="button"
          >
            <p className="text-sm font-semibold text-slate-950/84">创建新协作</p>
            <p className="mt-1 text-xs leading-5 text-slate-700/58">
              先整理任务，再决定需要哪些成员和上下文。
            </p>
          </button>
        </div>

        <div className="mt-6 flex-1 space-y-3 overflow-y-auto pr-1">
          {messages.map((message, index) => (
            <div
              key={`${message.role}-${index}`}
              className={cn(
                "rounded-[26px] px-4 py-4 text-sm leading-6 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)]",
                message.role === "app"
                  ? "bg-[rgba(255,255,255,0.08)] text-slate-900/82"
                  : "ml-auto max-w-[86%] bg-[rgba(130,148,255,0.14)] text-slate-950/86",
              )}
            >
              <div className="mb-2 flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
                {message.role === "app" ? <Sparkles className="h-3.5 w-3.5" /> : null}
                {message.role === "app" ? "App Agent" : "You"}
              </div>
              <p>{message.body}</p>
            </div>
          ))}
        </div>

        <div className="mt-6 space-y-3">
          <HeroInputShell class_name="w-full">
            <div className="flex min-w-0 items-center gap-3">
              <input
                className="flex-1 bg-transparent text-sm text-slate-900/84 outline-none placeholder:text-slate-700/42"
                onChange={(event) => {
                  set_draft(event.target.value);
                  on_change_draft(event.target.value);
                }}
                onKeyDown={(event) => {
                  if (event.key !== "Enter") {
                    return;
                  }

                  event.preventDefault();
                  on_submit(draft);
                }}
                placeholder="告诉 App 你要组织什么..."
                value={draft}
              />
              <button
                className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-white/84 text-slate-900 shadow-[0_10px_20px_rgba(255,255,255,0.16)] transition-transform duration-300 hover:-translate-y-0.5"
                onClick={() => on_submit(draft)}
                type="button"
              >
                <ArrowRight className="h-4 w-4" />
              </button>
            </div>
          </HeroInputShell>

          <button
            className="flex w-full items-center justify-between rounded-[22px] bg-white/8 px-4 py-4 text-left shadow-[inset_0_0_0_1px_rgba(255,255,255,0.1)] transition hover:bg-white/14"
            onClick={on_open_contacts_page}
            type="button"
          >
            <div className="flex items-start gap-3">
              <span className="mt-0.5 inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/12">
                <Users className="h-4 w-4 text-slate-900/76" />
              </span>
              <div>
                <p className="text-sm font-semibold text-slate-950/84">去 Contacts 选择成员</p>
                <p className="mt-1 text-xs leading-5 text-slate-700/58">
                  先看成员能力，再回到首页让 App 组织协作。
                </p>
              </div>
            </div>
            <ArrowRight className="h-4 w-4 shrink-0 text-slate-700/44" />
          </button>
        </div>
      </div>
    </HeroSidePanelShell>
  );
}
