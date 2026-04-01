/**
 * Activity Page - 完整的活动中心页面
 */

import { useEffect, useState } from 'react';
import { Filter, Search } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { AppRouteBuilders } from '@/app/router/route-paths';
import { WorkspaceEntryPage } from '@/shared/ui/workspace/workspace-entry-page';
import { getActivityEvents, getUnreadActivityCount, markActivityAsRead } from '@/lib/activity-api';
import { cn } from '@/lib/utils';
import { ActivityEvent, ActivityEventType } from '@/types/activity';

export function ActivityPage() {
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedFilter, setSelectedFilter] = useState<ActivityEventType | 'all'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const navigate = useNavigate();

  const loadEvents = async () => {
    setIsLoading(true);
    try {
      const loadedEvents = await getActivityEvents({ limit: 100 });
      setEvents(loadedEvents);

      const count = await getUnreadActivityCount();
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

  const filteredEvents = events.filter(event => {
    if (selectedFilter !== 'all' && event.event_type !== selectedFilter) {
      return false;
    }
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      const summary = (event.summary || '').toLowerCase();
      return summary.includes(query);
    }
    return true;
  });

  const handleFilterChange = (filter: ActivityEventType | 'all') => {
    setSelectedFilter(filter);
  };

  const handleMarkAsRead = async (eventIds: string[]) => {
    try {
      const unreadIds = eventIds.filter(
        eventId => events.some(event => event.id === eventId && !event.metadata_json?.read),
      );
      if (unreadIds.length === 0) {
        return;
      }

      await markActivityAsRead(eventIds);
      setEvents(prev =>
        prev.map(event =>
          eventIds.includes(event.id)
            ? { ...event, metadata_json: { ...(event.metadata_json || {}), read: true } }
            : event,
        ),
      );
      setUnreadCount(prev => Math.max(0, prev - unreadIds.length));
    } catch (error) {
      console.error('Failed to mark as read:', error);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="h-6 w-6 border-2 border-purple-200 border-t-transparent animate-spin rounded-full" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-linear-to-br from-slate-50 to-purple-50/10">
      {/* 页面头部 */}
      <div className="border-b border-slate-200 bg-white/80 backdrop-blur-sm">
        <div className="mx-auto max-w-4xl px-4 py-4 sm:px-6 sm:py-6">
          <div className="flex items-center justify-between gap-4">
            <h1 className="text-xl font-semibold text-slate-900">活动中心</h1>

            {/* 操作栏 */}
            <div className="flex items-center gap-2">
              {/* 搜索框 */}
              <div className="relative">
                <Search className="absolute left-3 h-4 w-4 text-slate-400" />
                <input
                  type="text"
                  placeholder="搜索活动..."
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                  className="pl-10 pr-4 py-1.5 text-sm border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500/50"
                />
              </div>

              {/* 筛选器 */}
              <select
                value={selectedFilter}
                onChange={e => handleFilterChange(e.target.value as ActivityEventType | 'all')}
                className="px-3 py-1.5 text-sm border border-slate-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-purple-500/50"
              >
                <option value="all">全部活动</option>
                <option value="agent_created">创建 Agent</option>
                <option value="room_created">创建 Room</option>
                <option value="room_message">Room 消息</option>
                <option value="room_round_completed">Room 完成</option>
                <option value="room_round_cancelled">Room 中断</option>
                <option value="room_round_failed">Room 失败</option>
                <option value="dm_message">DM 消息</option>
                <option value="permission_requested">权限请求</option>
                <option value="permission_resolved">权限决策</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      {/* 活动列表 */}
      <div className="mx-auto max-w-4xl px-4 py-6 sm:px-6 sm:py-8">
        {filteredEvents.length === 0 ? (
          <WorkspaceEntryPage
            title="暂无活动"
            description="当前没有活动记录。"
            icon={<Filter className="h-8 w-8 text-slate-400" />}
          />
        ) : (
          <div className="space-y-3">
            {/* 批量操作栏 */}
            {unreadCount > 0 && (
              <button
                onClick={() => handleMarkAsRead(filteredEvents.filter(e => !e.metadata_json?.read).map(e => e.id))}
                className="w-full py-2 px-4 rounded-lg bg-purple-600 text-white text-sm font-medium hover:bg-purple-700 transition-colors"
              >
                标记所有为已读
              </button>
            )}

            {/* 活动列表 */}
            {filteredEvents.map(event => (
              <div
                key={event.id}
                className={cn(
                  'group flex items-start gap-4 px-4 py-3 rounded-lg border transition-all',
                  event.metadata_json?.read
                    ? 'bg-slate-50 border-slate-100 opacity-60'
                    : 'bg-white border-slate-200 hover:border-purple-200 hover:shadow-sm',
                )}
              >
                {/* 事件类型图标 */}
                <div className={cn(
                  'flex h-9 w-9 shrink-0 items-center justify-center rounded-full',
                  'bg-linear-to-br from-purple-500 to-purple-600 text-white',
                )}>
                  {event.event_type === 'agent_created' && <span className="text-sm">🤖</span>}
                  {event.event_type === 'room_created' && <span className="text-sm">🏠</span>}
                  {event.event_type === 'room_message' && <span className="text-sm">💬</span>}
                  {event.event_type === 'room_round_completed' && <span className="text-sm">✅</span>}
                  {event.event_type === 'room_round_cancelled' && <span className="text-sm">⏹️</span>}
                  {event.event_type === 'room_round_failed' && <span className="text-sm">⚠️</span>}
                  {event.event_type === 'dm_message' && <span className="text-sm">💭</span>}
                  {event.event_type === 'permission_requested' && <span className="text-sm">🛂</span>}
                  {event.event_type === 'permission_resolved' && <span className="text-sm">🛡️</span>}
                  {event.event_type === 'skill_installed' && <span className="text-sm">📦</span>}
                  {event.event_type === 'skill_uninstalled' && <span className="text-sm">🗑️</span>}
                  {event.event_type === 'task_completed' && <span className="text-sm">✅</span>}
                  {event.event_type === 'task_failed' && <span className="text-sm">❌</span>}
                </div>

                {/* 事件内容 */}
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-semibold text-slate-900">
                      {event.summary || '系统活动'}
                    </p>
                    <span className="text-xs text-slate-400">
                      {new Date(event.created_at).toLocaleDateString('zh-CN', {
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </span>
                  </div>
                  {!event.metadata_json?.read && (
                    <button
                      onClick={() => handleMarkAsRead([event.id])}
                      className="text-xs text-purple-600 hover:text-purple-700"
                      type="button"
                    >
                      标记为已读
                    </button>
                  )}
                </div>

                {/* 跳转按钮 */}
                {(event.target_type === 'room' || event.target_type === 'agent') && event.target_id && (
                  <button
                    onClick={() => {
                      const targetId = event.target_id;
                      if (!targetId) {
                        return;
                      }

                      if (event.target_type === 'room') {
                        navigate(AppRouteBuilders.room(targetId));
                      } else {
                        navigate(AppRouteBuilders.contact_profile(targetId));
                      }
                    }}
                    className="px-3 py-1.5 text-sm border border-slate-200 rounded-md hover:bg-slate-50 transition-colors"
                    type="button"
                  >
                    查看详情
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
