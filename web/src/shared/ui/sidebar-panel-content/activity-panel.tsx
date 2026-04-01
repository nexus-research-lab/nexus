/**
 * Activity Panel 组件 - 显示在侧边栏的活动通知列表
 */

import { Bell, CheckCircle, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AppRouteBuilders } from '@/app/router/route-paths';
import { getActivityEvents, getUnreadActivityCount, markActivityAsRead } from '@/lib/activity-api';
import { cn } from '@/lib/utils';
import { ActivityEvent, ActivityItem } from '@/types/activity';

function getEventIcon(event_type: string) {
  switch (event_type) {
    case 'agent_created':
      return '🤖';
    case 'agent_updated':
      return '⚙️';
    case 'room_created':
      return '🏠';
    case 'room_message':
      return '💬';
    case 'room_round_completed':
      return '✅';
    case 'room_round_cancelled':
      return '⏹️';
    case 'room_round_failed':
      return '⚠️';
    case 'dm_message':
      return '💭';
    case 'permission_requested':
      return '🛂';
    case 'permission_resolved':
      return '🛡️';
    case 'skill_installed':
      return '📦';
    case 'skill_uninstalled':
      return '🗑️';
    case 'task_completed':
      return '✅';
    case 'task_failed':
      return '❌';
    default:
      return '🔔';
  }
}

function buildActivityItem(event: ActivityEvent): ActivityItem {
  const icon = getEventIcon(event.event_type);
  const title = event.summary || '';
  let subtitle: string | undefined;

  if (event.actor_type === 'user') {
    subtitle = '你';
  } else if (event.actor_type === 'agent') {
    subtitle = event.actor_id || 'Agent';
  } else {
    subtitle = '系统';
  }
  const tool_name = typeof event.metadata_json?.tool_name === 'string'
    ? event.metadata_json.tool_name
    : null;
  if (tool_name) {
    subtitle = subtitle ? `${subtitle} · ${tool_name}` : tool_name;
  }

  // 根据事件类型构建可点击的链接
  let action_url: string | undefined;
  if (event.target_type === 'room' && event.target_id) {
    action_url = AppRouteBuilders.room(event.target_id);
  } else if (event.target_type === 'agent' && event.target_id) {
    action_url = AppRouteBuilders.contact_profile(event.target_id);
  }

  return {
    event,
    icon,
    title,
    subtitle,
    action_url,
  };
}

function ActivityItemComponent({
  item,
  on_mark_as_read,
  on_open,
}: {
  item: ActivityItem;
  on_mark_as_read: (event_ids: string[]) => Promise<void>;
  on_open: (item: ActivityItem) => void;
}) {
  const [isLoading, setIsLoading] = useState(false);

  const handleClick = async () => {
    on_open(item);
  };

  const handleMarkAsRead = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (item.event.actor_type === 'user' || item.event.metadata_json?.read) return;

    setIsLoading(true);
    try {
      await on_mark_as_read([item.event.id]);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div
      className={cn(
        'group flex items-start gap-3 px-3 py-2.5 rounded-lg',
        'hover:bg-slate-100/50 transition-colors cursor-pointer',
      )}
      onClick={handleClick}
    >
      {/* 图标 */}
      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-linear-to-br from-purple-500/20 to-purple-600/20 text-white shadow-sm">
        <span className="text-sm">{item.icon}</span>
      </div>

      {/* 内容 */}
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm font-medium text-slate-900 line-clamp-1">{item.title}</p>
          <span className="text-xs text-slate-400">
            {new Date(item.event.created_at).toLocaleDateString('zh-CN', {
              month: 'short',
              day: 'numeric',
            })}
          </span>
        </div>
        {item.subtitle && (
          <p className="text-xs text-slate-600">{item.subtitle}</p>
        )}
      </div>

      {/* 标记为已读按钮 */}
      {item.event.actor_type !== 'user' && !item.event.metadata_json?.read && (
        <button
          className={cn(
            'h-6 w-6 shrink-0 rounded-full flex items-center justify-center',
            'text-slate-300 hover:text-slate-500 hover:bg-slate-200 transition-colors',
          )}
          onClick={handleMarkAsRead}
          disabled={isLoading}
          aria-label="标记为已读"
          type="button"
        >
          {isLoading ? (
            <div className="h-3 w-3 border-2 border-slate-300 border-t-transparent animate-spin rounded-full" />
          ) : (
            <CheckCircle className="h-3 w-3" />
          )}
        </button>
      )}
    </div>
  );
}

export function ActivityPanelContent() {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [showAll, setShowAll] = useState(false);
  const navigate = useNavigate();

  const loadEvents = async () => {
    setIsLoading(true);
    try {
      const [loadedEvents, count] = await Promise.all([
        getActivityEvents({ limit: 50 }),
        getUnreadActivityCount(),
      ]);

      setItems(loadedEvents.map(buildActivityItem));
      setUnreadCount(count);
    } catch (error) {
      console.error('Failed to load activity events:', error);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadEvents();
  }, []);

  const markEventsAsRead = async (eventIds: string[]) => {
    const unreadIds = eventIds.filter(
      eventId => items.some(item => item.event.id === eventId && !item.event.metadata_json?.read),
    );
    if (unreadIds.length === 0) return;

    await markActivityAsRead(unreadIds);
    setItems(prev =>
      prev.map(item =>
        unreadIds.includes(item.event.id)
          ? {
            ...item,
            event: {
              ...item.event,
              metadata_json: {
                ...(item.event.metadata_json || {}),
                read: true,
              },
            },
          }
          : item,
      ),
    );
    setUnreadCount(prev => Math.max(0, prev - unreadIds.length));
  };

  const handleMarkAllAsRead = async () => {
    const unreadIds = items
      .filter(item => item.event.actor_type !== 'user' && !item.event.metadata_json?.read)
      .map(item => item.event.id);
    await markEventsAsRead(unreadIds);
  };

  const handleOpenItem = (item: ActivityItem) => {
    if (!item.action_url) {
      return;
    }
    navigate(item.action_url);
  };

  const displayItems = showAll
    ? items
    : items.filter(item => item.event.actor_type === 'user' || item.event.actor_type === 'agent');

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="h-6 w-6 border-2 border-purple-200 border-t-transparent animate-spin rounded-full" />
      </div>
    );
  }

  return (
    <div className="flex flex-col min-h-0">
      {/* 头部 */}
      <div className="flex items-center justify-between px-3 py-2.5 border-b border-slate-200">
        <h2 className="text-sm font-semibold text-slate-900">活动中心</h2>
        {unreadCount > 0 && (
          <button
            className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-purple-600 text-white text-xs font-medium hover:bg-purple-700 transition-colors"
            onClick={handleMarkAllAsRead}
            type="button"
          >
            <Bell className="h-3 w-3" />
            <span>{unreadCount}</span>
            <X className="h-3 w-3 ml-1" />
          </button>
        )}
      </div>

      {/* 活动列表 */}
      <div className="flex-1 overflow-y-auto soft-scrollbar">
        {displayItems.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-center py-12">
            <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-slate-100">
              <Bell className="h-5 w-5 text-slate-400" />
            </div>
            <p className="text-sm font-semibold text-slate-600">暂无活动</p>
            <p className="max-w-[180px] text-xs leading-5 text-slate-400">
              当有新的通知、权限请求或系统事件时，会在这里显示。
            </p>
          </div>
        ) : (
          displayItems.map(item => (
            <ActivityItemComponent
              key={item.event.id}
              item={item}
              on_mark_as_read={markEventsAsRead}
              on_open={handleOpenItem}
            />
          ))
        )}

        {!showAll && displayItems.length < items.length && (
          <button
            className="w-full py-2 text-sm text-purple-700 hover:text-purple-900 transition-colors border-t border-slate-200"
            onClick={() => setShowAll(true)}
          >
            显示更多
          </button>
        )}
      </div>
    </div>
  );
}
