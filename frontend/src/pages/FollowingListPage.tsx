import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ArrowLeft, User } from 'lucide-react';
import { getFollowingList } from '../api/relation';
import { ErrorCode } from '../types/api';
import type { FollowListItem } from '../types/relation';
import LoadingSpinner from '../components/LoadingSpinner';
import ErrorMessage from '../components/ErrorMessage';
import EmptyState from '../components/EmptyState';

export default function FollowingListPage() {
  const { userId } = useParams<{ userId: string }>();
  const [items, setItems] = useState<FollowListItem[]>([]);
  const [total, setTotal] = useState(0);
  const [cursor, setCursor] = useState<string | undefined>();
  const [hasMore, setHasMore] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async (cursorVal?: string) => {
    if (!userId) return;
    setIsLoading(true);
    setError(null);
    try {
      const res = await getFollowingList(userId, cursorVal);
      if (res.code === ErrorCode.Success) {
        if (cursorVal) {
          setItems((prev) => [...prev, ...res.data.list]);
        } else {
          setItems(res.data.list);
        }
        setTotal(res.data.total);
        setCursor(res.data.next_cursor);
        setHasMore(!!res.data.next_cursor);
      } else {
        setError(res.msg);
      }
    } catch (err: any) {
      setError(err.message || '加载失败');
    }
    setIsLoading(false);
  };

  useEffect(() => { fetchData(); }, [userId]);

  return (
    <div>
      <button
        onClick={() => window.history.back()}
        className="flex items-center gap-1 text-sm text-scholar-muted hover:text-scholar-text mb-4 transition-colors"
      >
        <ArrowLeft className="w-4 h-4" />
        返回
      </button>

      <h1 className="text-2xl font-bold text-scholar-text mb-1">关注列表</h1>
      <p className="text-sm text-scholar-muted mb-6">共 {total} 人</p>

      {isLoading && items.length === 0 ? (
        <LoadingSpinner />
      ) : error ? (
        <ErrorMessage message={error} onRetry={() => fetchData()} />
      ) : items.length === 0 ? (
        <EmptyState title="还没有关注任何人" description="去看看有什么有趣的人吧" />
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <Link
              key={item.user_id}
              to={`/users/${item.user_id}/following`}
              className="flex items-center gap-3 bg-white border border-scholar-border rounded-lg p-4 hover:border-scholar-accent/30 transition-colors"
            >
              <div className="w-10 h-10 rounded-full bg-scholar-accent/10 flex items-center justify-center text-scholar-accent font-bold">
                <User className="w-5 h-5" />
              </div>
              <div>
                <div className="font-medium text-scholar-text text-sm">{item.username}</div>
                <div className="text-xs text-scholar-muted">
                  {new Date(item.created_at).toLocaleDateString('zh-CN')} 开始关注
                </div>
              </div>
            </Link>
          ))}
          {hasMore && (
            <div className="flex justify-center pt-4">
              <button
                onClick={() => fetchData(cursor)}
                disabled={isLoading}
                className="px-6 py-2 text-sm text-scholar-accent border border-scholar-accent/30 rounded-lg hover:bg-scholar-accent/5 disabled:opacity-50 transition-colors"
              >
                {isLoading ? '加载中...' : '加载更多'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
