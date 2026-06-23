/**
 * AskUserQuestion Block Component - 用户问答交互组件
 *
 * 渲染 Agent 的问题，支持单选/多选，用户提交答案后返回给 Agent
 *
 * [INPUT]: 依赖 @/types/conversation/ask-user-question、@/types/conversation/message、@/lib/utils
 * [OUTPUT]: 对外提供 AskUserQuestionBlock 组件
 * [POS]: block 模块的专用工具组件，被 content-renderer.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useState, useCallback, useEffect, useMemo } from 'react';
import { AlertCircle, Check, CheckCircle, ChevronDown, ChevronRight, MessageSquare, Send } from 'lucide-react';
import { cn } from '@/lib/utils';
import {
    AskUserQuestionInput,
    UserQuestionAnswer,
    is_ask_user_question_timed_out_result,
} from '@/types/conversation/ask-user-question';
import { ToolResultContent, ToolUseContent } from '@/types/conversation/message';
import { MessageRail } from "../ui/message-rail";
import { AskUserQuestionCard } from "./ask-user-question-card";
import {
    build_submitted_selection_state,
    create_empty_question_selection_state,
    has_selection_state_content,
    normalize_question,
} from "./ask-user-question-model";

interface AskUserQuestionBlockProps {
    tool_use: ToolUseContent;
    tool_result?: ToolResultContent;
    on_submit?: (tool_use_id: string, answers: UserQuestionAnswer[]) => boolean | Promise<boolean>;
    is_submitted?: boolean;
    is_ready?: boolean;
    interaction_disabled?: boolean;
    interaction_disabled_reason?: string;
}

export function AskUserQuestionBlock({
    tool_use,
    tool_result,
    on_submit,
    is_submitted: initialSubmitted = false,
    is_ready = true,
    interaction_disabled = false,
    interaction_disabled_reason,
}: AskUserQuestionBlockProps) {
    // 解析输入
    const input = tool_use.input as AskUserQuestionInput;
    const questions = useMemo(
        () => (input?.questions || []).map(normalize_question),
        [input?.questions],
    );
    const submitted_selection_state = useMemo(
        () => build_submitted_selection_state(questions, tool_result),
        [questions, tool_result],
    );
    const has_submitted_selection_state = useMemo(
        () => has_selection_state_content(submitted_selection_state),
        [submitted_selection_state],
    );

    // 状态：每个问题的选中选项
    const [selections, setSelections] = useState<Map<number, Set<string>>>(
        () => create_empty_question_selection_state(questions).selections,
    );
    const [customAnswers, setCustomAnswers] = useState<Map<number, string>>(
        () => create_empty_question_selection_state(questions).custom_answers,
    );
    const isTimedOut = is_ask_user_question_timed_out_result(tool_result);
    const isFailed = Boolean(tool_result?.is_error && !isTimedOut);
    const [hasLocalSubmission, setHasLocalSubmission] = useState(false);
    const isSubmitted = initialSubmitted || hasLocalSubmission;
    const isObserverReadOnly = interaction_disabled && !isSubmitted && !isTimedOut && !isFailed;
    const shouldStartCollapsed = initialSubmitted || isTimedOut || isFailed;
    // 展开/收起状态：首帧就按最终状态初始化，避免先展开再收起的闪动
    const [isExpanded, setIsExpanded] = useState(() => !shouldStartCollapsed);

    useEffect(() => {
        if (initialSubmitted || isTimedOut || isFailed) {
            setIsExpanded(false);
        }
    }, [initialSubmitted, isFailed, isTimedOut]);

    useEffect(() => {
        const empty_state = create_empty_question_selection_state(questions);
        setSelections(empty_state.selections);
        setCustomAnswers(empty_state.custom_answers);
    }, [questions]);

    useEffect(() => {
        if (!initialSubmitted && !has_submitted_selection_state) {
            return;
        }
        setSelections(new Map(
            Array.from(submitted_selection_state.selections.entries()).map(([index, values]) => [index, new Set(values)]),
        ));
        setCustomAnswers(new Map(submitted_selection_state.custom_answers));
    }, [has_submitted_selection_state, initialSubmitted, submitted_selection_state]);

    // 切换选项
    const handleToggleOption = useCallback((questionIndex: number, optionLabel: string, multiSelect: boolean) => {
        if (isSubmitted) return;

        setSelections(prev => {
            const newMap = new Map(prev);
            const currentSet = new Set(prev.get(questionIndex) || []);

            if (multiSelect) {
                // 多选：切换选中状态
                if (currentSet.has(optionLabel)) {
                    currentSet.delete(optionLabel);
                } else {
                    currentSet.add(optionLabel);
                }
            } else {
                // 单选：清空后选中
                currentSet.clear();
                currentSet.add(optionLabel);
            }

            newMap.set(questionIndex, currentSet);
            return newMap;
        });

        if (!multiSelect) {
            setCustomAnswers((prev) => {
                const nextMap = new Map(prev);
                nextMap.set(questionIndex, '');
                return nextMap;
            });
        }
    }, [isSubmitted]);

    const handleCustomAnswerChange = useCallback((
        questionIndex: number,
        customAnswer: string,
        multiSelect: boolean,
    ) => {
        if (isSubmitted) return;

        setCustomAnswers((prev) => {
            const nextMap = new Map(prev);
            nextMap.set(questionIndex, customAnswer);
            return nextMap;
        });

        if (!multiSelect && customAnswer.trim()) {
            setSelections((prev) => {
                const nextMap = new Map(prev);
                nextMap.set(questionIndex, new Set());
                return nextMap;
            });
        }
    }, [isSubmitted]);

    // 检查是否可以提交（每个问题至少选一个）
    const canSubmit = useMemo(() => {
        return questions.every((_, index) => {
            const selected = selections.get(index);
            const customAnswer = customAnswers.get(index)?.trim() || '';
            return (selected && selected.size > 0) || customAnswer.length > 0;
        });
    }, [customAnswers, questions, selections]);

    // 提交回答
    const handleSubmit = useCallback(async () => {
        if (!canSubmit || isSubmitted || !is_ready || interaction_disabled) return;

        const answers: UserQuestionAnswer[] = questions.map((_, index) => {
            const selectedOptions = Array.from(selections.get(index) || []);
            const customAnswer = customAnswers.get(index)?.trim() || '';
            if (customAnswer) {
                selectedOptions.push(customAnswer);
            }

            return {
                question_index: index,
                selected_options: selectedOptions,
            };
        });

        const submitted = await on_submit?.(tool_use.id, answers);
        if (submitted === false) {
            return;
        }
        setHasLocalSubmission(true);
        setIsExpanded(false); // 提交后收起
    }, [canSubmit, customAnswers, interaction_disabled, isSubmitted, is_ready, on_submit, questions, selections, tool_use.id]);

    // 计算已选数量
    const totalSelected = useMemo(() => {
        let count = 0;
        selections.forEach((set, index) => {
            count += set.size;
            if (customAnswers.get(index)?.trim()) {
                count += 1;
            }
        });
        return count;
    }, [customAnswers, selections]);

    // 获取回答摘要（收起时显示）
    const answerSummary = useMemo(() => {
        if (!isSubmitted) return null;
        const allSelected: string[] = [];
        selections.forEach((set, index) => {
            set.forEach(label => allSelected.push(label));
            const customAnswer = customAnswers.get(index)?.trim();
            if (customAnswer) {
                allSelected.push(customAnswer);
            }
        });
        return allSelected.slice(0, 3).join('、') + (allSelected.length > 3 ? '...' : '');
    }, [customAnswers, isSubmitted, selections]);

    if (questions.length === 0) {
        return null;
    }

    const isReadOnly = isSubmitted || isTimedOut || isFailed || isObserverReadOnly;
    const headerToneClassName = isTimedOut || isFailed
        ? "text-(--warning)"
        : isSubmitted
            ? "text-(--success)"
            : "text-primary";
    const headerLabel = isTimedOut
        ? "提问已超时"
            : isFailed
                ? "提问未完成"
            : isObserverReadOnly
                ? "等待回应"
                : isSubmitted
                    ? "已收到你的回应"
                    : "需要你的回应";

    return (
        <MessageRail class_name="my-1.5">
            {/* ═══════════ 头部（可点击展开/收起） ═══════════ */}
            <div
                className={cn(
                    "flex min-h-8 cursor-pointer select-none items-center gap-2 py-0.5 text-xs transition duration-(--motion-duration-fast) ease-out",
                )}
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <div className={cn(
                    "flex h-5 w-5 items-center justify-center rounded-full",
                    headerToneClassName,
                )} data-timeline-anchor data-timeline-anchor-mode="box">
                    {isTimedOut || isFailed ? (
                        <AlertCircle className="w-3.5 h-3.5" />
                    ) : isSubmitted ? (
                        <CheckCircle className="w-3.5 h-3.5" />
                    ) : (
                        <MessageSquare className="w-3.5 h-3.5" />
                    )}
                </div>

                <span className={cn(
                    "font-medium uppercase tracking-[0.12em]",
                    headerToneClassName,
                )}>
                    {headerLabel}
                </span>

                <span className="text-muted-foreground/30">│</span>

                <span className="text-muted-foreground">
                    {questions.length} 个问题
                </span>

                {/* 收起时显示回答摘要 */}
                {!isExpanded && answerSummary && (
                    <>
                        <span className="text-muted-foreground/30">│</span>
                        <span className="truncate max-w-[200px] text-(--text-muted)">
                            {answerSummary}
                        </span>
                    </>
                )}

                <div className="flex-1" />

                {!isReadOnly && totalSelected > 0 && (
                    <span className="text-[10px] font-semibold text-primary/80">
                        已选 {totalSelected} 项
                    </span>
                )}

                {/* 展开/收起指示器 */}
                <div className="text-muted-foreground/40">
                    {isExpanded ? (
                        <ChevronDown className="w-3.5 h-3.5" />
                    ) : (
                        <ChevronRight className="w-3.5 h-3.5" />
                    )}
                </div>
            </div>

            {/* ═══════════ 问题列表（可收起） ═══════════ */}
            {isExpanded && (
                <div className="mt-2 space-y-2">
                    {questions.map((question, index) => (
                        <AskUserQuestionCard
                            key={index}
                            question={question}
                            question_index={index}
                            selected_options={selections.get(index) || new Set()}
                            custom_answer={customAnswers.get(index) || ''}
                            on_toggle_option={handleToggleOption}
                            on_custom_answer_change={handleCustomAnswerChange}
                            is_submitted={isReadOnly}
                            default_expanded={!isReadOnly}
                        />
                    ))}
                </div>
            )}

            {/* ═══════════ 底部操作栏 ═══════════ */}
            {!isReadOnly && isExpanded && (
                <div className="message-cjk-font mt-2 flex min-h-0 items-center justify-between gap-3 border-t border-(--divider-subtle-color) pt-2">
                    <span className="text-[11px] leading-none text-muted-foreground">
                        {!is_ready
                            ? '等待提问就绪'
                            : isObserverReadOnly
                                ? (interaction_disabled_reason || '当前暂不可操作')
                                : canSubmit
                                    ? '✓ 所有问题都已回应'
                                    : '每个问题至少回应一次'}
                    </span>

                    <button
                        onClick={(e) => {
                            e.stopPropagation();
                            void handleSubmit();
                        }}
                        disabled={!canSubmit || !is_ready || interaction_disabled}
                        title={interaction_disabled ? interaction_disabled_reason : undefined}
                        className={cn(
                            "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-[8px] border px-2.5 py-1 text-xs font-medium leading-none transition-colors",
                            canSubmit && is_ready && !interaction_disabled
                                ? "border-primary/24 bg-primary/8 text-primary hover:bg-primary/12"
                                : "border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
                        )}
                    >
                        <Send className="h-3 w-3" />
                        继续协作
                    </button>
                </div>
            )}

            {/* ═══════════ 已提交状态（展开时显示） ═══════════ */}
            {isSubmitted && isExpanded && (
                <div className="message-cjk-font mt-2 flex items-center gap-2 border-t border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] pt-2 text-xs font-semibold text-(--success)">
                    <Check className="w-3.5 h-3.5 text-(--success)" />
                    <span className="text-xs font-medium text-(--success)">已收到你的回应</span>
                </div>
            )}
        </MessageRail>
    );
}
