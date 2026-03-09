"use client";

import { useState, useMemo } from "react";
import { Search, X, SlidersHorizontal, Calendar, Tag, Star } from "lucide-react";
import { cn } from "@/lib/utils";

interface Session {
    id: string;
    metadata: {
        title: string;
        tags: string[];
        starred: boolean;
    };
    createdAt: number;
    updatedAt: number;
}

interface SessionSearchProps {
    sessions: Session[];
    onSelect: (sessionId: string) => void;
    currentSessionId: string | null;
}

type SortBy = 'recent' | 'oldest' | 'title';
type FilterBy = 'all' | 'starred' | 'tagged';

export function SessionSearch({ sessions, onSelect, currentSessionId }: SessionSearchProps) {
    const [searchQuery, setSearchQuery] = useState("");
    const [showFilters, setShowFilters] = useState(false);
    const [sortBy, setSortBy] = useState<SortBy>('recent');
    const [filterBy, setFilterBy] = useState<FilterBy>('all');
    const [selectedTags, setSelectedTags] = useState<string[]>([]);

    // 提取所有标签
    const allTags = useMemo(() => {
        const tagSet = new Set<string>();
        sessions.forEach(session => {
            session.metadata.tags.forEach(tag => tagSet.add(tag));
        });
        return Array.from(tagSet);
    }, [sessions]);

    // 过滤和排序会话
    const filteredSessions = useMemo(() => {
        let result = [...sessions];

        // 搜索过滤
        if (searchQuery.trim()) {
            const query = searchQuery.toLowerCase();
            result = result.filter(session =>
                session.metadata.title.toLowerCase().includes(query)
            );
        }

        // 标签过滤
        if (selectedTags.length > 0) {
            result = result.filter(session =>
                selectedTags.some(tag => session.metadata.tags.includes(tag))
            );
        }

        // 状态过滤
        if (filterBy === 'starred') {
            result = result.filter(session => session.metadata.starred);
        } else if (filterBy === 'tagged') {
            result = result.filter(session => session.metadata.tags.length > 0);
        }

        // 排序
        result.sort((a, b) => {
            switch (sortBy) {
                case 'recent':
                    return b.updatedAt - a.updatedAt;
                case 'oldest':
                    return a.createdAt - b.createdAt;
                case 'title':
                    return a.metadata.title.localeCompare(b.metadata.title);
                default:
                    return 0;
            }
        });

        return result;
    }, [sessions, searchQuery, sortBy, filterBy, selectedTags]);

    const toggleTag = (tag: string) => {
        setSelectedTags(prev =>
            prev.includes(tag)
                ? prev.filter(t => t !== tag)
                : [...prev, tag]
        );
    };

    return (
        <div className="flex flex-col h-full">
            {/* 搜索栏 */}
            <div className="p-4 border-b border-primary/20">
                <div className="relative">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground/50" size={18} />
                    <input
                        type="text"
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        placeholder="搜索对话..."
                        className={cn(
                            "w-full pl-10 pr-10 py-2 rounded-lg",
                            "bg-background/50 border border-primary/30",
                            "focus:outline-none focus:ring-1 focus:ring-primary/50",
                            "text-sm placeholder:text-muted-foreground/50",
                            "transition-all"
                        )}
                    />
                    {searchQuery && (
                        <button
                            onClick={() => setSearchQuery("")}
                            className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground/50 hover:text-primary transition-colors"
                        >
                            <X size={16} />
                        </button>
                    )}
                </div>

                {/* 过滤器切换 */}
                <div className="flex items-center gap-2 mt-3">
                    <button
                        onClick={() => setShowFilters(!showFilters)}
                        className={cn(
                            "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium",
                            "border transition-all",
                            showFilters
                                ? "bg-primary/10 border-primary/50 text-primary"
                                : "bg-background/50 border-primary/20 text-muted-foreground hover:border-primary/40"
                        )}
                    >
                        <SlidersHorizontal size={14} />
                        过滤器
                    </button>

                    {/* 快速过滤 */}
                    <button
                        onClick={() => setFilterBy(filterBy === 'starred' ? 'all' : 'starred')}
                        className={cn(
                            "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium",
                            "border transition-all",
                            filterBy === 'starred'
                                ? "bg-yellow-500/10 border-yellow-500/50 text-yellow-500"
                                : "bg-background/50 border-primary/20 text-muted-foreground hover:border-primary/40"
                        )}
                    >
                        <Star size={14} />
                        收藏
                    </button>

                    <div className="flex-1" />

                    <span className="text-xs text-muted-foreground/50">
                        {filteredSessions.length} / {sessions.length}
                    </span>
                </div>
            </div>

            {/* 过滤器面板 */}
            {showFilters && (
                <div className="p-4 border-b border-primary/20 bg-background/30 space-y-3">
                    {/* 排序 */}
                    <div>
                        <label className="text-xs font-medium text-muted-foreground mb-2 block">排序方式</label>
                        <div className="flex gap-2">
                            {[
                                { value: 'recent', label: '最近', icon: Calendar },
                                { value: 'oldest', label: '最早', icon: Calendar },
                                { value: 'title', label: '标题', icon: Tag },
                            ].map(({ value, label, icon: Icon }) => (
                                <button
                                    key={value}
                                    onClick={() => setSortBy(value as SortBy)}
                                    className={cn(
                                        "flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium",
                                        "border transition-all",
                                        sortBy === value
                                            ? "bg-primary/10 border-primary/50 text-primary"
                                            : "bg-background border-primary/20 text-muted-foreground hover:border-primary/40"
                                    )}
                                >
                                    <Icon size={12} />
                                    {label}
                                </button>
                            ))}
                        </div>
                    </div>

                    {/* 标签过滤 */}
                    {allTags.length > 0 && (
                        <div>
                            <label className="text-xs font-medium text-muted-foreground mb-2 block">标签</label>
                            <div className="flex flex-wrap gap-1.5">
                                {allTags.map(tag => (
                                    <button
                                        key={tag}
                                        onClick={() => toggleTag(tag)}
                                        className={cn(
                                            "px-2 py-1 rounded-md text-xs font-medium",
                                            "border transition-all",
                                            selectedTags.includes(tag)
                                                ? "bg-primary/10 border-primary/50 text-primary"
                                                : "bg-background border-primary/20 text-muted-foreground hover:border-primary/40"
                                        )}
                                    >
                                        #{tag}
                                    </button>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            )}

            {/* 结果列表 */}
            <div className="flex-1 overflow-y-auto">
                {filteredSessions.length === 0 ? (
                    <div className="flex flex-col items-center justify-center h-full text-center p-8">
                        <Search size={48} className="text-muted-foreground/30 mb-3" />
                        <p className="text-sm text-muted-foreground/50">
                            {searchQuery ? "未找到匹配的对话" : "暂无对话"}
                        </p>
                    </div>
                ) : (
                    <div className="p-2 space-y-1">
                        {filteredSessions.map(session => (
                            <button
                                key={session.id}
                                onClick={() => onSelect(session.id)}
                                className={cn(
                                    "w-full text-left p-3 rounded-lg transition-all",
                                    "border border-transparent",
                                    currentSessionId === session.id
                                        ? "bg-primary/10 border-primary/50"
                                        : "hover:bg-primary/5 hover:border-primary/30"
                                )}
                            >
                                <div className="flex items-start gap-2">
                                    {session.metadata.starred && (
                                        <Star size={12} className="text-yellow-500 mt-1 flex-shrink-0" fill="currentColor" />
                                    )}
                                    <div className="flex-1 min-w-0">
                                        <div className="text-sm font-medium truncate">
                                            {session.metadata.title}
                                        </div>
                                        {session.metadata.tags.length > 0 && (
                                            <div className="flex flex-wrap gap-1 mt-1">
                                                {session.metadata.tags.map(tag => (
                                                    <span
                                                        key={tag}
                                                        className="text-[10px] px-1.5 py-0.5 rounded bg-primary/10 text-primary"
                                                    >
                                                        #{tag}
                                                    </span>
                                                ))}
                                            </div>
                                        )}
                                        <div className="text-[10px] text-muted-foreground/50 mt-1">
                                            {new Date(session.updatedAt).toLocaleString('zh-CN', {
                                                month: 'short',
                                                day: 'numeric',
                                                hour: '2-digit',
                                                minute: '2-digit',
                                            })}
                                        </div>
                                    </div>
                                </div>
                            </button>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
}
