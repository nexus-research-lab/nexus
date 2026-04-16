/**
 * AskUserQuestion Block Component - 用户问答交互组件
 *
 * 渲染 Claude 的问题，支持单选/多选，用户提交答案后返回给 Agent
 *
 * [INPUT]: 依赖 @/types/conversation/ask-user-question、@/types/conversation/message、@/lib/utils
 * [OUTPUT]: 对外提供 AskUserQuestionBlock 组件
 * [POS]: block 模块的专用工具组件，被 content-renderer.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useState, useCallback, useEffect, useMemo } from 'react';
import { AlertCircle, Check, CheckCircle, CheckSquare, ChevronDown, ChevronRight, Circle, MessageSquare, Send, Square } from 'lucide-react';
import { cn } from '@/lib/utils';
import {
    AskUserQuestionInput,
    UserQuestion,
    UserQuestionAnswer,
    is_ask_user_question_timed_out_result,
} from '@/types/conversation/ask-user-question';
import { ToolResultContent, ToolUseContent } from '@/types/conversation/message';
import { MessageRail } from "../ui/message-rail";

interface AskUserQuestionCardProps {
    question: UserQuestion;
    question_index: number;
    selected_options: Set<string>;
    custom_answer: string;
    on_toggle_option: (question_index: number, option_label: string, multi_select: boolean) => void;
    on_custom_answer_change: (question_index: number, custom_answer: string, multi_select: boolean) => void;
    is_submitted: boolean;
    default_expanded?: boolean;
}

interface AskUserQuestionBlockProps {
    tool_use: ToolUseContent;
    tool_result?: ToolResultContent;
    on_submit?: (tool_use_id: string, answers: UserQuestionAnswer[]) => boolean | Promise<boolean>;
    is_submitted?: boolean;
    is_ready?: boolean;
    interaction_disabled?: boolean;
    interaction_disabled_reason?: string;
}

function normalize_question(question: UserQuestion): UserQuestion {
    return {
        ...question,
        // 兼容 SDK 直接透传的 camelCase 字段，组件内部统一使用 snake_case。
        multi_select: question.multi_select ?? question.multiSelect ?? false,
    };
}

// ==================== 子组件 ====================

/** 单个问题卡片（支持独立收起） */
function QuestionCard({
    question,
    question_index,
    selected_options,
    custom_answer,
    on_toggle_option,
    on_custom_answer_change,
    is_submitted,
    default_expanded = false,
}: AskUserQuestionCardProps) {
    const [isExpanded, setIsExpanded] = useState(default_expanded);
    const isMultiSelect = question.multi_select ?? false;
    const hasCustomAnswer = custom_answer.trim().length > 0;
    const showCustomAnswer = !is_submitted || hasCustomAnswer;
    const hasSelection = selected_options.size > 0 || hasCustomAnswer;
    const selectedCount = selected_options.size + (hasCustomAnswer ? 1 : 0);

    // 选中摘要（收起时显示）
    const summaryItems = [...Array.from(selected_options), ...(hasCustomAnswer ? [custom_answer.trim()] : [])];
    const selectionSummary = summaryItems.slice(0, 2).join('、') +
        (summaryItems.length > 2 ? '...' : '');

    return (
        <div className={cn(
            "overflow-hidden rounded-[14px] border transition duration-(--motion-duration-fast) ease-out",
            hasSelection
                ? "border-primary/18 bg-white/10"
                : "border-white/12 bg-white/5",
        )}>
            {/* 问题头部（可点击收起） */}
            <div
                className={cn(
                    "flex cursor-pointer select-none items-center gap-1 px-2.5 py-1 transition duration-(--motion-duration-fast) ease-out",
                    isExpanded && "border-b border-white/12",
                    !isExpanded && "hover:bg-white/6",
                )}
                onClick={() => setIsExpanded(!isExpanded)}
            >
                {/* 序号 */}
                <span className={cn(
                    "flex h-5 w-5 items-center justify-center rounded-full bg-(--material-chip-background) text-[10px] font-bold text-(--text-muted)",
                    hasSelection && "bg-primary/12 text-primary",
                )}>
                    {question_index + 1}
                </span>

                {/* header 标签 */}
                {question.header && (
                    <span className="rounded-xl bg-primary/8 px-1.5 text-[9px] font-bold uppercase tracking-[0.08em] text-primary/80">
                        {question.header}
                    </span>
                )}

                {/* 问题文本 */}
                <span className="flex-1 truncate text-[13px] font-medium leading-tight text-foreground">
                    {question.question}
                </span>

                {isMultiSelect && (
                    <span className="text-[10px] text-muted-foreground">(多选)</span>
                )}

                {/* 收起时显示选中摘要 */}
                {!isExpanded && hasSelection && (
                    <span className="text-xs text-primary/70 truncate max-w-[120px]">
                        {selectionSummary}
                    </span>
                )}

                {/* 选中数量 */}
                {hasSelection && (
                    <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-semibold text-primary">
                        {selectedCount}
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

            {/* 选项列表（可收起） */}
            {isExpanded && (
                <div className="space-y-1.5 p-2.5">
                    {question.options.map((option, optIndex) => {
                        const isSelected = selected_options.has(option.label);
                        const Icon = isMultiSelect
                            ? (isSelected ? CheckSquare : Square)
                            : (isSelected ? CheckCircle : Circle);

                        return (
                            <button
                                key={optIndex}
                                className={cn(
                                    "w-full rounded-[12px] border px-2.5 py-0.5 text-left transition duration-(--motion-duration-fast) ease-out",
                                    isSelected
                                        ? "border-primary/20 bg-primary/6"
                                        : "border-white/12 bg-white/5 hover:border-primary/16 hover:bg-primary/4",
                                    is_submitted && "cursor-not-allowed opacity-60",
                                )}
                                disabled={is_submitted}
                                onClick={(e) => {
                                    e.stopPropagation();
                                    on_toggle_option(question_index, option.label, isMultiSelect);
                                }}
                            >
                                <div className="flex items-start gap-2.5">
                                    <Icon className={cn(
                                        "mt-0.5 h-4 w-4 flex-shrink-0 transition-colors",
                                        isSelected ? "text-primary" : "text-muted-foreground/50"
                                    )} />
                                    <div className="flex-1 min-w-0">
                                        <div className={cn(
                                            "text-[13px] font-medium leading-tight",
                                            isSelected ? "text-primary" : "text-foreground"
                                        )}>
                                            {option.label}
                                        </div>
                                        {option.description && (
                                            <div className="mt-0.5 text-[11px] leading-snug text-muted-foreground">
                                                {option.description}
                                            </div>
                                        )}
                                    </div>
                                    {isSelected && (
                                        <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                                            已选
                                        </span>
                                    )}
                                </div>
                            </button>
                        );
                    })}

                    {showCustomAnswer ? (
                        <div
                            className="px-0.5 pt-1"
                            onClick={(event) => event.stopPropagation()}
                        >
                            <div className="mb-1 flex items-center justify-between gap-2">
                                <div className="text-[13px] font-medium text-foreground">自定义回答</div>
                                {hasCustomAnswer && (
                                    <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                                        已填写
                                    </span>
                                )}
                            </div>
                            <div className="border-b border-white/14">
                                <textarea
                                    className={cn(
                                        "h-7 min-h-7 w-full resize-none border-0 bg-transparent px-0 py-0 text-[13px] leading-7 text-(--text-strong) outline-none shadow-none ring-0 transition duration-(--motion-duration-fast) ease-out focus:border-0 focus:bg-transparent focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
                                        "placeholder:text-muted-foreground/70",
                                        is_submitted && "cursor-not-allowed opacity-60"
                                    )}
                                    disabled={is_submitted}
                                    value={custom_answer}
                                    onChange={(event) => {
                                        on_custom_answer_change(
                                            question_index,
                                            event.target.value,
                                            isMultiSelect,
                                        );
                                    }}
                                    onClick={(event) => event.stopPropagation()}
                                    placeholder={isMultiSelect ? "可补充其他答案…" : "没有合适选项时，在这里输入你的回答…"}
                                    rows={1}
                                />
                            </div>
                        </div>
                    ) : null}
                </div>
            )}
        </div>
    );
}

// ==================== 主组件 ====================

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

    // 状态：每个问题的选中选项
    const [selections, setSelections] = useState<Map<number, Set<string>>>(() => {
        const map = new Map();
        questions.forEach((_, index) => map.set(index, new Set()));
        return map;
    });
    const [customAnswers, setCustomAnswers] = useState<Map<number, string>>(() => {
        const map = new Map();
        questions.forEach((_, index) => map.set(index, ''));
        return map;
    });
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
                ? "等待另一窗口回应"
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
                )}>
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
                    <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-semibold text-primary">
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
                        <QuestionCard
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
                <div className="flex min-h-0 items-center justify-between gap-3 border-t border-white/12">
                    <span className="text-[11px] leading-none text-muted-foreground">
                        {!is_ready
                            ? '等待提问就绪'
                            : isObserverReadOnly
                                ? (interaction_disabled_reason || '当前窗口是观察视图')
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
                            "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-3 py-1 text-xs font-medium leading-none transition-colors",
                            canSubmit && is_ready && !interaction_disabled
                                ? "border-primary/24 bg-primary/10 text-primary hover:bg-primary/14"
                                : "border-white/14 bg-white/6 text-(--text-soft)",
                        )}
                    >
                        <Send className="h-3 w-3" />
                        继续协作
                    </button>
                </div>
            )}

            {/* ═══════════ 已提交状态（展开时显示） ═══════════ */}
            {isSubmitted && isExpanded && (
                <div className="mt-2 flex items-center gap-2 border-t border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] pt-2 text-xs font-semibold text-(--success)">
                    <Check className="w-3.5 h-3.5 text-(--success)" />
                    <span className="text-xs font-medium text-(--success)">已收到你的回应</span>
                </div>
            )}
        </MessageRail>
    );
}
