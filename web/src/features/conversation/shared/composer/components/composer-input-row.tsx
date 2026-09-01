// INPUT: Composer 草稿、mention/slash picker 状态与 textarea 交互。
// OUTPUT: 保留草稿的输入区和就近只读 picker 反馈。
// POS: Composer 输入行展示边界；不执行 Session 设置 mutation。
import { useRef } from "react";
import type {
  ClipboardEventHandler,
  KeyboardEvent,
  KeyboardEventHandler,
  RefObject,
  UIEvent,
  WheelEvent,
} from "react";

import { cn } from "@/shared/ui/class-name";
import type { MentionTargetItem } from "@/shared/ui/mention/mention-target-model";
import { MentionTargetPopover } from "@/shared/ui/mention/mention-target-popover";
import type {
  CommandCatalogStatus,
  CommandDescriptor,
} from "@/types/generated/protocol";
import type { SkillInfo } from "@/types/capability/skill";
import { projectLeadingSlashCommand } from "../../slash-command-presentation";
import { SlashCommandToken } from "../../slash-command-token";

import { COMPOSER_TEXTAREA_MAX_HEIGHT_PX } from "../composer-styles";
import type { SlashModelOption } from "../slash-command-model";
import type { ComposerReadFailure } from "../controller/composer-settings-reliability";
import { SlashCommandPopover } from "./slash-command-popover";

interface ComposerInputRowProps {
  input: {
    disabled: boolean;
    onChange: (value: string) => void;
    onCompositionEnd: (timeStamp: number) => void;
    onCompositionStart: () => void;
    onKeyDown: KeyboardEventHandler<HTMLTextAreaElement>;
    onPaste: ClipboardEventHandler<HTMLTextAreaElement>;
    placeholder: string;
    value: string;
  };
  layout: {
    paddingClassName: string;
  };
  mention: {
    active: boolean;
    filter: string;
    items: MentionTargetItem[];
    onClose: () => void;
    onSelect: (item: MentionTargetItem) => void;
  };
  slashCommand: {
    active: boolean;
    activeIndex: number;
    commands: CommandDescriptor[];
    mode: "commands" | "models" | "skills";
    modelError: ComposerReadFailure | null;
    modelItems: SlashModelOption[];
    modelLoading: boolean;
    modelQuery: string;
    modelSearchRef: RefObject<HTMLInputElement | null>;
    onModelQueryChange: (query: string) => void;
    onModelQueryKeyDown: (
      event: KeyboardEvent<HTMLInputElement>,
    ) => boolean;
    onModelRetry: () => void;
    onClose: () => void;
    onSelectModel: (model: SlashModelOption) => void;
    onSelectCommand: (command: CommandDescriptor) => void;
    onSelectSkill: (skill: SkillInfo) => void;
    onSkillQueryChange: (query: string) => void;
    onSkillQueryKeyDown: (
      event: KeyboardEvent<HTMLInputElement>,
    ) => boolean;
    onSkillRetry: () => void;
    skillError: ComposerReadFailure | null;
    skillItems: SkillInfo[];
    skillLoading: boolean;
    skillQuery: string;
    skillSearchRef: RefObject<HTMLInputElement | null>;
    status: CommandCatalogStatus;
  };
  composerShellRef: RefObject<HTMLDivElement | null>;
  textareaRef: RefObject<HTMLTextAreaElement | null>;
}

/** 中文注释：输入行只保留文字区；附件、模式与发送等动作统一收在底部工具行。 */
export function ComposerInputRow({
  input,
  layout,
  mention,
  slashCommand,
  composerShellRef,
  textareaRef,
}: ComposerInputRowProps) {
  const slashCommandPresentation = projectLeadingSlashCommand(input.value);
  const slashCommandMirrorRef = useRef<HTMLDivElement>(null);
  const handleScroll = (event: UIEvent<HTMLTextAreaElement>) => {
    if (slashCommandMirrorRef.current) {
      slashCommandMirrorRef.current.scrollTop = event.currentTarget.scrollTop;
    }
  };

  return (
    <div className={cn("flex items-end gap-2", layout.paddingClassName)}>
      {slashCommand.active ? (
        <SlashCommandPopover
          activeIndex={slashCommand.activeIndex}
          anchorRef={composerShellRef}
          commands={slashCommand.commands}
          mode={slashCommand.mode}
          modelError={slashCommand.modelError}
          modelItems={slashCommand.modelItems}
          modelLoading={slashCommand.modelLoading}
          modelQuery={slashCommand.modelQuery}
          modelSearchRef={slashCommand.modelSearchRef}
          onModelQueryChange={slashCommand.onModelQueryChange}
          onModelQueryKeyDown={slashCommand.onModelQueryKeyDown}
          onModelRetry={slashCommand.onModelRetry}
          onClose={slashCommand.onClose}
          onSelectModel={slashCommand.onSelectModel}
          onSelectCommand={slashCommand.onSelectCommand}
          onSelectSkill={slashCommand.onSelectSkill}
          onSkillQueryChange={slashCommand.onSkillQueryChange}
          onSkillQueryKeyDown={slashCommand.onSkillQueryKeyDown}
          onSkillRetry={slashCommand.onSkillRetry}
          skillError={slashCommand.skillError}
          skillItems={slashCommand.skillItems}
          skillLoading={slashCommand.skillLoading}
          skillQuery={slashCommand.skillQuery}
          skillSearchRef={slashCommand.skillSearchRef}
          status={slashCommand.status}
        />
      ) : mention.active && mention.items.length > 0 ? (
        <MentionTargetPopover
          anchorRect={textareaRef.current?.getBoundingClientRect() ?? null}
          filter={mention.filter}
          items={mention.items}
          onClose={mention.onClose}
          onSelect={mention.onSelect}
          placement="above"
        />
      ) : null}
      <div className="relative min-w-0 flex-1">
        {slashCommandPresentation ? (
          <div
            ref={slashCommandMirrorRef}
            aria-hidden="true"
            className={cn(
              "pointer-events-none absolute inset-0 overflow-hidden px-1.5 py-1 text-base leading-6 whitespace-pre-wrap text-(--text-strong) [overflow-wrap:break-word]",
              input.disabled && "opacity-(--disabled-opacity)",
            )}
            data-composer-slash-command="true"
          >
            <SlashCommandToken variant="composer">
              {slashCommandPresentation.command}
            </SlashCommandToken>
            {slashCommandPresentation.remainder}
          </div>
        ) : null}
        <textarea
          ref={textareaRef}
          aria-label={input.placeholder}
          className={cn(
            "multiline-cursor soft-scrollbar relative z-10 block min-h-8 w-full min-w-0 resize-none overflow-y-auto overscroll-contain bg-transparent px-1.5 py-1 text-base leading-6 outline-none shadow-none ring-0",
            slashCommandPresentation ? "text-transparent" : "text-(--text-strong)",
            "placeholder:text-(--text-soft)",
            "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
            "focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
          )}
          disabled={input.disabled}
          onChange={(event) => input.onChange(event.target.value)}
          onCompositionEnd={(event) => input.onCompositionEnd(event.timeStamp)}
          onCompositionStart={input.onCompositionStart}
          onKeyDown={input.onKeyDown}
          onPaste={input.onPaste}
          onScroll={handleScroll}
          onWheel={stopNestedTextareaWheel}
          placeholder={input.placeholder}
          rows={1}
          style={{
            caretColor: slashCommandPresentation
              ? "var(--text-strong)"
              : undefined,
            maxHeight: COMPOSER_TEXTAREA_MAX_HEIGHT_PX,
          }}
          value={input.value}
        />
      </div>
    </div>
  );
}

function stopNestedTextareaWheel(event: WheelEvent<HTMLTextAreaElement>) {
  const target = event.currentTarget;
  if (target.scrollHeight > target.clientHeight) {
    event.stopPropagation();
  }
}
