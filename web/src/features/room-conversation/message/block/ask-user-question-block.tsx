/**
 * AskUserQuestion Block Component - 用户问答交互组件
 *
 * 渲染 Claude 的问题，支持单选/多选，用户提交答案后返回给 Agent
 *
 * [INPUT]: 依赖 @/types/ask-user-question、@/types/message、@/lib/utils
 * [OUTPUT]: 对外提供 AskUserQuestionBlock 组件
 * [POS]: block 模块的专用工具组件，被 content-renderer.tsx 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

"use client";

import { useState, useCallback, useMemo } from 'react';
import { Check, CheckCircle, CheckSquare, ChevronDown, ChevronRight, Circle, MessageSquare, Send, Square } from 'lucide-react';
import { cn } from '@/lib/utils';
import { AskUserQuestionInput, UserQuestion, UserQuestionAnswer } from '@/types/ask-user-question';
import { ToolUseContent } from '@/types/message';

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
  on_submit?: (tool_use_id: string, answers: UserQuestionAnswer[]) => boolean | Promise<boolean>;
  is_submitted?: boolean;
  is_ready?: boolean;
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
    const hasSelection = selected_options.size > 0 || hasCustomAnswer;
    const selectedCount = selected_options.size + (hasCustomAnswer ? 1 : 0);

    // 选中摘要（收起时显示）
    const summaryItems = [...Array.from(selected_options), ...(hasCustomAnswer ? [custom_answer.trim()] : [])];
    const selectionSummary = summaryItems.slice(0, 2).join('、') +
        (summaryItems.length > 2 ? '...' : '');

    return (
        <div className={cn(
            "overflow-hidden rounded-[18px] border transition-all duration-200",
            hasSelection
                ? "border-white/32 bg-white/20 shadow-[0_14px_24px_rgba(133,119,255,0.08)]"
                : "border-white/20 bg-white/10"
        )}>
            {/* 问题头部（可点击收起） */}
            <div
                className={cn(
                    "flex items-center gap-2 px-3 py-2 cursor-pointer select-none",
                    "transition-colors hover:bg-white/12",
                    isExpanded && "border-b border-white/18"
                )}
                onClick={() => setIsExpanded(!isExpanded)}
            >
                {/* 序号 */}
                <span className={cn(
                    "w-5 h-5 flex items-center justify-center rounded-full text-[10px] font-bold",
                    hasSelection ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"
                )}>
                    {question_index + 1}
                </span>

                {/* header 标签 */}
                {question.header && (
                    <span className="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-primary/80">
                        {question.header}
                    </span>
                )}

                {/* 问题文本 */}
                <span className="text-sm font-medium text-foreground truncate flex-1">
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
                    <span className="rounded-full bg-primary/20 px-1.5 py-0.5 text-[10px] font-medium text-primary">
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
                <div className="p-3 space-y-2">
                    {question.options.map((option, optIndex) => {
                        const isSelected = selected_options.has(option.label);
                        const Icon = isMultiSelect
                            ? (isSelected ? CheckSquare : Square)
                            : (isSelected ? CheckCircle : Circle);

                        return (
                            <button
                                key={optIndex}
                                disabled={is_submitted}
                                onClick={(e) => {
                                    e.stopPropagation();
                                    on_toggle_option(question_index, option.label, isMultiSelect);
                                }}
                                className={cn(
                                    "radius-shell-sm w-full border text-left p-3 transition-all duration-200",
                                    "hover:border-primary/50 hover:bg-primary/5",
                                    isSelected
                                        ? "border-white/32 bg-primary/10 shadow-[0_12px_20px_rgba(133,119,255,0.08)]"
                                        : "border-white/18 bg-white/8",
                                    is_submitted && "opacity-60 cursor-not-allowed"
                                )}
                            >
                                <div className="flex items-start gap-3">
                                    <Icon className={cn(
                                        "w-4 h-4 mt-0.5 flex-shrink-0 transition-colors",
                                        isSelected ? "text-primary" : "text-muted-foreground/50"
                                    )} />
                                    <div className="flex-1 min-w-0">
                                        <div className={cn(
                                            "text-sm font-medium",
                                            isSelected ? "text-primary" : "text-foreground"
                                        )}>
                                            {option.label}
                                        </div>
                                        {option.description && (
                                            <div className="text-xs text-muted-foreground mt-0.5 leading-relaxed">
                                                {option.description}
                                            </div>
                                        )}
                                    </div>
                                    {isSelected && (
                                        <span className="rounded-full bg-primary/20 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                                            已选
                                        </span>
                                    )}
                                </div>
                            </button>
                        );
                    })}

                    <div
                        className={cn(
                            "radius-shell-sm border p-3 transition-all duration-200",
                            hasCustomAnswer
                                ? "border-white/32 bg-primary/10 shadow-[0_12px_20px_rgba(133,119,255,0.08)]"
                                : "border-white/18 bg-white/8"
                        )}
                        onClick={(event) => event.stopPropagation()}
                    >
                        <div className="mb-2 flex items-center justify-between gap-2">
                            <div className="text-sm font-medium text-foreground">自定义回答</div>
                            {hasCustomAnswer && (
                                <span className="rounded-full bg-primary/20 px-1.5 py-0.5 text-[10px] font-medium text-primary">
                                    已填写
                                </span>
                            )}
                        </div>
                        <textarea
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
                            rows={3}
                            className={cn(
                                "w-full resize-none rounded-2xl border border-white/36 bg-white/58 px-3 py-2 text-sm text-foreground outline-none transition-all",
                                "placeholder:text-muted-foreground/70 focus:border-primary/40 focus:bg-white/75",
                                is_submitted && "cursor-not-allowed opacity-60"
                            )}
                        />
                    </div>
                </div>
            )}
        </div>
    );
}

// ==================== 主组件 ====================

export function AskUserQuestionBlock({
    tool_use,
    on_submit,
    is_submitted: initialSubmitted = false,
    is_ready = true,
}: AskUserQuestionBlockProps) {
    // 解析输入
    const input = tool_use.input as AskUserQuestionInput;
    const questions = input?.questions || [];

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
    const [isSubmitted, setIsSubmitted] = useState(initialSubmitted);
    // 展开/收起状态：已提交时默认收起
    const [isExpanded, setIsExpanded] = useState(!initialSubmitted);

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
        if (!canSubmit || isSubmitted || !is_ready) return;

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
        setIsSubmitted(true);
        setIsExpanded(false); // 提交后收起
    }, [canSubmit, customAnswers, isSubmitted, is_ready, on_submit, questions, selections, tool_use.id]);

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

    return (
        <div className={cn(
            "my-2 overflow-hidden rounded-[22px] border transition-all duration-300",
            isSubmitted
                ? "border-white/28 bg-white/14 shadow-[0_18px_30px_rgba(102,217,143,0.08)]"
                : "border-white/24 bg-white/12 shadow-[0_18px_30px_rgba(133,119,255,0.08)]"
        )}>
            {/* ═══════════ 头部（可点击展开/收起） ═══════════ */}
            <div
                className={cn(
                    "flex min-h-11 items-center gap-2 px-3 text-xs cursor-pointer select-none transition-colors",
                    "hover:bg-white/10",
                    isSubmitted ? "border-green-500/20" : "border-primary/20",
                    isExpanded && "border-b"
                )}
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <div className={cn(
                    "flex h-7 w-7 items-center justify-center rounded-full border border-white/28 bg-white/20",
                    isSubmitted ? "text-green-500" : "text-primary"
                )}>
                    {isSubmitted ? (
                        <CheckCircle className="w-3.5 h-3.5" />
                    ) : (
                        <MessageSquare className="w-3.5 h-3.5" />
                    )}
                </div>

                <span className={cn(
                    "font-medium uppercase tracking-[0.14em]",
                    isSubmitted ? "text-green-500" : "text-primary"
                )}>
                    {isSubmitted ? '已确认' : '需要你的回应'}
                </span>

                <span className="text-muted-foreground/30">│</span>

                <span className="text-muted-foreground">
                    {questions.length} 个问题
                </span>

                {/* 收起时显示回答摘要 */}
                {!isExpanded && answerSummary && (
                    <>
                        <span className="text-muted-foreground/30">│</span>
                        <span className="text-muted-foreground/60 truncate max-w-[200px]">
                            {answerSummary}
                        </span>
                    </>
                )}

                <div className="flex-1" />

                {!isSubmitted && totalSelected > 0 && (
                    <span className="rounded-full bg-primary/20 px-1.5 py-0.5 text-[10px] text-primary">
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
                <div className="space-y-6 p-4">
                    {questions.map((question, index) => (
                        <QuestionCard
                            key={index}
                            question={question}
                            question_index={index}
                            selected_options={selections.get(index) || new Set()}
                            custom_answer={customAnswers.get(index) || ''}
                            on_toggle_option={handleToggleOption}
                            on_custom_answer_change={handleCustomAnswerChange}
                            is_submitted={isSubmitted}
                        />
                    ))}
                </div>
            )}

            {/* ═══════════ 底部操作栏 ═══════════ */}
            {!isSubmitted && isExpanded && (
                <div className="flex h-12 items-center justify-between border-t border-white/18 bg-white/14 px-4">
                    <span className="text-xs text-muted-foreground">
                        {!is_ready
                            ? '等待系统准备提问上下文'
                            : canSubmit
                                ? '✓ 所有问题都已回应'
                                : '每个问题至少回应一次'}
                    </span>

                    <button
                        onClick={(e) => {
                            e.stopPropagation();
                            void handleSubmit();
                        }}
                        disabled={!canSubmit || !is_ready}
                        className={cn(
                            "radius-shell-sm flex items-center gap-2 px-4 py-1.5 text-xs font-medium transition-all duration-200",
                            canSubmit && is_ready
                                ? "bg-primary text-primary-foreground hover:bg-primary/90 shadow-[0_14px_24px_rgba(133,119,255,0.18)]"
                                : "neo-pill text-muted-foreground cursor-not-allowed"
                        )}
                    >
                        <Send className="w-3 h-3" />
                        继续协作
                    </button>
                </div>
            )}

            {/* ═══════════ 已提交状态（展开时显示） ═══════════ */}
            {isSubmitted && isExpanded && (
                <div className="flex h-10 items-center gap-2 border-t border-green-500/20 bg-green-500/5 px-4">
                    <Check className="w-3.5 h-3.5 text-green-500" />
                    <span className="text-xs font-medium text-green-600">已收到你的回应</span>
                </div>
            )}
        </div>
    );
}
