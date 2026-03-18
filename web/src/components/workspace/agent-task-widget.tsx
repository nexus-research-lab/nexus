"use client";

import { AnimatePresence, motion } from "framer-motion";
import { CheckCircle2, Circle, Clock, Sparkles, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useState } from "react";

export interface TodoItem {
  content: string;
  status: "pending" | "completed" | "in_progress";
  activeForm?: string;
}

interface AgentTaskWidgetProps {
  todos: TodoItem[];
}

export function AgentTaskWidget({todos}: AgentTaskWidgetProps) {
  const [isOpen, setIsOpen] = useState(false);

  if (todos.length === 0) return null;

  const activeCount = todos.filter(t => t.status === 'in_progress').length;
  const completedCount = todos.filter(t => t.status === 'completed').length;
  const totalCount = todos.length;
  const progress = totalCount > 0 ? (completedCount / totalCount) * 100 : 0;
  const isWorking = activeCount > 0;

  return (
    <div className="relative">
      <motion.button
        onClick={() => setIsOpen(!isOpen)}
        whileHover={{scale: 1.1}}
        whileTap={{scale: 0.95}}
        className={cn(
          "neo-pill radius-shell-sm relative h-8 w-8 overflow-hidden",
          "hover:text-primary hover:shadow-[0_12px_20px_rgba(133,119,255,0.18)]",
          isOpen && "text-primary shadow-[0_16px_26px_rgba(133,119,255,0.22)]",
          "transition-all duration-300"
        )}
      >
        <div
          className="absolute inset-0 opacity-20"
          style={{
            backgroundImage: "linear-gradient(rgba(133,119,255,0.08) 1px, transparent 1px), linear-gradient(90deg, rgba(133,119,255,0.08) 1px, transparent 1px)",
            backgroundSize: "4px 4px"
          }}
        />

        {/* 进度环 */}
        <svg className="absolute inset-0 -rotate-90 w-full h-full p-0.5" viewBox="0 0 32 32">
          <circle
            cx="16" cy="16" r="14"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            className="text-primary/20"
          />
          <motion.circle
            cx="16" cy="16" r="14"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            className="text-primary"
            initial={{pathLength: 0}}
            animate={{pathLength: progress / 100}}
            transition={{duration: 0.5}}
            style={{
              filter: "drop-shadow(0 0 3px rgba(133,119,255,0.45))"
            }}
          />
        </svg>

        {/* 中心图标 */}
        <div className="absolute inset-0 flex items-center justify-center">
          {isWorking ? (
            <motion.div
              animate={{rotate: 360}}
              transition={{duration: 4, repeat: Infinity, ease: "linear"}}
            >
              <Sparkles className="w-3.5 h-3.5 text-primary"
                        style={{filter: "drop-shadow(0 0 4px rgba(133,119,255,0.45))"}}/>
            </motion.div>
          ) : (
            <CheckCircle2 className="w-3.5 h-3.5 text-primary/70"/>
          )}
        </div>

        {/* 工作中的扫描线效果 */}
        {isWorking && (
          <motion.div
            className="absolute inset-0 bg-gradient-to-b from-primary/20 via-transparent to-transparent"
            animate={{y: ["-100%", "100%"]}}
            transition={{duration: 1.5, repeat: Infinity, ease: "linear"}}
          />
        )}

      </motion.button>

      <AnimatePresence>
        {isOpen && (
          <>
            {/* Backdrop */}
            <motion.div
              initial={{opacity: 0}}
              animate={{opacity: 1}}
              exit={{opacity: 0}}
              className="fixed inset-0 z-40"
              onClick={() => setIsOpen(false)}
            />

            <motion.div
              initial={{opacity: 0, y: -8, scale: 0.96}}
              animate={{opacity: 1, y: 0, scale: 1}}
              exit={{opacity: 0, y: -8, scale: 0.96}}
              transition={{type: "spring", stiffness: 400, damping: 30}}
              className={cn(
                "absolute top-full right-0 mt-3 w-72",
                "soft-ring radius-shell-lg panel-surface overflow-hidden z-50"
              )}
            >
              {/* Header */}
              <div className="relative p-4 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="neo-pill radius-shell-sm p-2 text-primary">
                    <Sparkles size={14}/>
                  </div>
                  <div>
                    <h3 className="font-semibold text-sm text-foreground">Agent Plan</h3>
                    <div className="flex items-center gap-2 text-[10px] text-muted-foreground">
                      <span>{completedCount}/{totalCount} done</span>
                      <div className="h-1 w-12 overflow-hidden rounded-full bg-muted/50">
                        <motion.div
                          className="h-full rounded-full bg-primary"
                          initial={{width: 0}}
                          animate={{width: `${progress}%`}}
                        />
                      </div>
                    </div>
                  </div>
                </div>
                <button
                  onClick={() => setIsOpen(false)}
                  className="neo-pill radius-shell-sm p-1.5 text-muted-foreground transition-colors hover:text-foreground"
                >
                  <X size={14}/>
                </button>
              </div>

              {/* Divider */}
              <div className="mx-4 h-px bg-gradient-to-r from-transparent via-border to-transparent"/>

              {/* Task List */}
              <div className="max-h-[320px] overflow-y-auto p-3 space-y-2 custom-scrollbar">
                {todos.map((todo, index) => (
                  <motion.div
                    key={`${index}-${todo.content}`}
                    initial={{opacity: 0, x: -10}}
                    animate={{opacity: 1, x: 0}}
                    transition={{delay: index * 0.03}}
                    className={cn(
                      "radius-shell-md relative p-3 transition-all duration-300",
                      todo.status === "in_progress"
                        ? "neo-card bg-primary/10 shadow-[0_14px_24px_rgba(133,119,255,0.12)]"
                        : todo.status === "completed"
                          ? "neo-card-flat opacity-55"
                          : "neo-card-flat"
                    )}
                  >
                    <div className="flex items-start gap-2.5">
                      <div className="mt-0.5 shrink-0">
                        {todo.status === "completed" ? (
                          <CheckCircle2 className="w-4 h-4 text-green-400"/>
                        ) : todo.status === "in_progress" ? (
                          <div className="relative">
                            <motion.div
                              className="absolute inset-0 rounded-full bg-primary"
                              animate={{scale: [1, 1.8, 1.8], opacity: [0.4, 0, 0]}}
                              transition={{duration: 1.5, repeat: Infinity}}
                            />
                            <Clock className="w-4 h-4 text-primary relative"/>
                          </div>
                        ) : (
                            <Circle className="w-4 h-4 text-muted-foreground/40"/>
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className={cn(
                          "text-xs leading-relaxed",
                          todo.status === "completed"
                            ? "line-through text-muted-foreground/60"
                            : "text-foreground/80"
                        )}>
                          {todo.content}
                        </p>
                        {todo.activeForm && todo.status === "in_progress" && (
                          <p className="text-[10px] text-primary/70 font-mono mt-1.5 flex items-center gap-1">
                            <motion.span
                              className="w-1 h-1 rounded-full bg-primary"
                              animate={{opacity: [1, 0.3, 1]}}
                              transition={{duration: 1, repeat: Infinity}}
                            />
                            {todo.activeForm}
                          </p>
                        )}
                      </div>
                    </div>
                  </motion.div>
                ))}
              </div>

              {/* 底部渐变 */}
              <div className="h-px bg-gradient-to-r from-transparent via-primary/20 to-transparent"/>
            </motion.div>
          </>
        )}
      </AnimatePresence>
    </div>
  );
}
