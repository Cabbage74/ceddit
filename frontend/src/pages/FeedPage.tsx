import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { PenLine } from 'lucide-react';
import { useFeedStore } from '../stores/feedStore';
import { useAuthStore } from '../stores/authStore';
import PostList from '../components/PostList';
import LoadingSpinner from '../components/LoadingSpinner';
import ErrorMessage from '../components/ErrorMessage';
import EmptyState from '../components/EmptyState';

export default function FeedPage() {
  const { items, isLoading, error, hasMore, fetchFeed, loadMore } = useFeedStore();
  const { isLoggedIn } = useAuthStore();
  const navigate = useNavigate();

  useEffect(() => {
    fetchFeed(1);
  }, [fetchFeed]);

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-scholar-text">首页</h1>
          <p className="text-sm text-scholar-muted mt-1">发现知识文章</p>
        </div>
        {isLoggedIn && (
          <button
            onClick={() => navigate('/create')}
            className="flex items-center gap-1.5 px-4 py-2 text-sm text-white bg-scholar-accent hover:bg-scholar-accentHover rounded-lg transition-colors"
          >
            <PenLine className="w-4 h-4" />
            写文章
          </button>
        )}
      </div>

      {/* Content */}
      {isLoading && items.length === 0 ? (
        <LoadingSpinner />
      ) : error ? (
        <ErrorMessage message={error} onRetry={() => fetchFeed(1)} />
      ) : items.length === 0 ? (
        <EmptyState
          title="还没有文章"
          description="成为第一个分享知识的人吧"
          action={isLoggedIn ? { label: '写文章', onClick: () => navigate('/create') } : undefined}
        />
      ) : (
        <PostList items={items} hasMore={hasMore} isLoading={isLoading} onLoadMore={loadMore} />
      )}
    </div>
  );
}
