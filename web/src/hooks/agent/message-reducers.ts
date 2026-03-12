/**
 * 消息处理纯函数
 *
 * 无状态的消息归并、流式事件处理、工具调用提取逻辑
 * 从 useAgentSession Hook 中抽取，保持纯函数与反应式逻辑的职责分离
 *
 * [INPUT]: 依赖 @/types 的 Message/StreamEvent/ToolCall 类型
 * [OUTPUT]: 对外提供 reduceIncomingMessage, extractToolCallsFromMessage, mergeToolCalls
 * [POS]: hooks/agent 模块的消息处理内核，被 useAgentSession 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { AssistantMessage, Message, StreamEvent, ToolCall } from '@/types';
import { generateUuid } from '@/lib/uuid';

// ==================== 流式事件判别 ====================

export function isStreamEventMessage(message: Message | StreamEvent): message is StreamEvent {
    return 'type' in message && !('role' in message);
}

// ==================== Content Block 操作 ====================

function getContentBlockKey(block: any): string | null {
    if (!block || typeof block !== 'object') {
        return null;
    }
    if (block.type === 'thinking') {
        return 'thinking';
    }
    if (block.type === 'tool_use' && block.id) {
        return `tool_use:${block.id}`;
    }
    if (block.type === 'tool_result' && block.tool_use_id) {
        return `tool_result:${block.tool_use_id}`;
    }
    if (block.type === 'text' && typeof block.text === 'string') {
        return `text:${block.text}`;
    }
    return null;
}

export function mergeAssistantContent(existingContent: any[], incomingContent: any[]): any[] {
    const merged = [...existingContent];
    const indexMap = new Map<string, number>();

    merged.forEach((block, index) => {
        const key = getContentBlockKey(block);
        if (key) {
            indexMap.set(key, index);
        }
    });

    incomingContent.forEach((block) => {
        const key = getContentBlockKey(block);
        if (!key) {
            merged.push(block);
            return;
        }

        const existingIndex = indexMap.get(key);
        if (existingIndex === undefined) {
            merged.push(block);
            indexMap.set(key, merged.length - 1);
            return;
        }

        merged[existingIndex] = block;
    });

    const thinkingIndex = merged.findIndex(block => block?.type === 'thinking');
    if (thinkingIndex > 0) {
        const [thinkingBlock] = merged.splice(thinkingIndex, 1);
        merged.unshift(thinkingBlock);
    }

    return merged;
}

// ==================== Assistant 消息定位 ====================

function findAssistantMessageIndex(messages: Message[], messageId?: string): number {
    if (messageId) {
        const exactIndex = messages.findIndex(
            msg => msg.role === 'assistant' && msg.message_id === messageId
        );
        if (exactIndex !== -1) {
            return exactIndex;
        }
    }

    for (let index = messages.length - 1; index >= 0; index -= 1) {
        if (messages[index].role === 'assistant') {
            return index;
        }
    }
    return -1;
}

// ==================== 流式消息处理 ====================

function createAssistantMessageFromStreamStart(
    event: StreamEvent,
    messageSessionKey: string,
    roundId: string
): AssistantMessage {
    return {
        message_id: event.message_id || generateUuid(),
        agent_id: messageSessionKey,
        round_id: roundId,
        role: 'assistant',
        content: [],
        timestamp: Date.now(),
        model: event.message?.model,
    };
}

function applyStreamEventToAssistantMessage(
    message: AssistantMessage,
    event: StreamEvent
): AssistantMessage {
    const updatedMessage: AssistantMessage = {
        ...message,
        content: [...message.content],
    };

    if (event.type === 'content_block_start' && event.content_block && typeof event.index === 'number') {
        updatedMessage.content[event.index] = event.content_block;
        return updatedMessage;
    }

    if (event.type === 'content_block_delta' && event.delta && typeof event.index === 'number') {
        const block = updatedMessage.content[event.index] as any;
        const delta = event.delta;
        if (!block) {
            return updatedMessage;
        }

        if (block.type === 'text' && delta.type === 'text_delta') {
            updatedMessage.content[event.index] = {
                ...block,
                text: block.text + (delta.text || '')
            };
            return updatedMessage;
        }

        if (block.type === 'thinking' && delta.type === 'thinking_delta') {
            updatedMessage.content[event.index] = {
                ...block,
                thinking: block.thinking + (delta.thinking || '')
            };
            return updatedMessage;
        }

        if (block.type === 'tool_use' && delta.type === 'input_json_delta') {
            try {
                updatedMessage.content[event.index] = {
                    ...block,
                    input: JSON.parse(delta.partial_json),
                };
            } catch {
                // 忽略不完整JSON，等待后续delta
            }
            return updatedMessage;
        }
    }

    if (event.type === 'message_delta') {
        if (event.delta?.stop_reason) {
            updatedMessage.stop_reason = event.delta.stop_reason;
        }
    }

    return updatedMessage;
}

function handleStreamEventMessage(
    messages: Message[],
    event: StreamEvent,
    messageSessionKey: string,
    roundId: string
): Message[] {
    if (event.type === 'message_start') {
        if (event.message_id) {
            const exists = messages.some(
                msg => msg.role === 'assistant' && msg.message_id === event.message_id
            );
            if (exists) {
                return messages;
            }
        }
        return [...messages, createAssistantMessageFromStreamStart(event, messageSessionKey, roundId)];
    }

    const targetIndex = findAssistantMessageIndex(messages, event.message_id);
    if (targetIndex === -1) {
        return messages;
    }

    const assistantMessage = messages[targetIndex] as AssistantMessage;
    const updatedMessage = applyStreamEventToAssistantMessage(assistantMessage, event);
    const nextMessages = [...messages];
    nextMessages[targetIndex] = updatedMessage;
    return nextMessages;
}

// ==================== 消息归并 ====================

function mergeToolResultMessage(messages: Message[], message: AssistantMessage): Message[] | null {
    if (!message.is_tool_result) {
        return null;
    }

    const toolResultBlock = message.content.find(
        (block): block is Extract<AssistantMessage['content'][number], { type: 'tool_result' }> =>
            block.type === 'tool_result'
    );
    if (!toolResultBlock || !toolResultBlock.tool_use_id) {
        return null;
    }

    const reverseIndex = [...messages].reverse().findIndex(msg =>
        msg.role === 'assistant' &&
        Array.isArray(msg.content) &&
        msg.content.some((block: any) => block.type === 'tool_use' && block.id === toolResultBlock.tool_use_id)
    );
    if (reverseIndex === -1) {
        return null;
    }

    const targetIndex = messages.length - 1 - reverseIndex;
    const targetMessage = messages[targetIndex] as AssistantMessage;
    const updatedMessage: AssistantMessage = {
        ...targetMessage,
        content: mergeAssistantContent(targetMessage.content, message.content),
    };

    const nextMessages = [...messages];
    nextMessages[targetIndex] = updatedMessage;
    return nextMessages;
}

function upsertMessageById(messages: Message[], message: Message): Message[] | null {
    const existingIndex = message.message_id
        ? messages.findIndex(item => item.message_id === message.message_id)
        : -1;
    if (existingIndex === -1) {
        return null;
    }

    const nextMessages = [...messages];
    if (message.role !== 'assistant' || nextMessages[existingIndex].role !== 'assistant') {
        nextMessages[existingIndex] = message;
        return nextMessages;
    }

    const existingAssistant = nextMessages[existingIndex] as AssistantMessage;
    const incomingAssistant = message as AssistantMessage;
    nextMessages[existingIndex] = {
        ...incomingAssistant,
        content: mergeAssistantContent(existingAssistant.content, incomingAssistant.content),
    };
    return nextMessages;
}

/**
 * 核心归并函数：将 incoming 消息（完整消息或流式事件）合入现有消息列表
 */
export function reduceIncomingMessage(
    messages: Message[],
    incoming: Message | StreamEvent,
    messageSessionKey: string,
    roundId: string
): Message[] {
    if (isStreamEventMessage(incoming)) {
        return handleStreamEventMessage(messages, incoming, messageSessionKey, roundId);
    }

    if (incoming.role === 'assistant') {
        const mergedToolResult = mergeToolResultMessage(messages, incoming);
        if (mergedToolResult) {
            return mergedToolResult;
        }
    }

    const upserted = upsertMessageById(messages, incoming);
    if (upserted) {
        return upserted;
    }

    return [...messages, incoming];
}

// ==================== 工具调用 ====================

export function extractToolCallsFromMessage(message: Message): ToolCall[] {
    if (message.role !== 'assistant' || !Array.isArray(message.content)) {
        return [];
    }
    return message.content
        .filter((block): block is Extract<AssistantMessage['content'][number], { type: 'tool_use' }> =>
            block.type === 'tool_use'
        )
        .map(block => ({
            id: block.id,
            tool_name: block.name,
            input: block.input || {},
            status: 'running',
            start_time: Date.now(),
        }));
}

export function mergeToolCalls(prev: ToolCall[], incoming: ToolCall[]): ToolCall[] {
    if (incoming.length === 0) {
        return prev;
    }
    const merged = new Map<string, ToolCall>();
    prev.forEach(call => merged.set(call.id, call));
    incoming.forEach(call => {
        const existing = merged.get(call.id);
        if (!existing) {
            merged.set(call.id, call);
            return;
        }
        merged.set(call.id, { ...existing, ...call });
    });
    return [...merged.values()];
}
