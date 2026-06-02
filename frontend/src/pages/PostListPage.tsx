import { useEffect, useState } from 'react';
import { getPostList } from '../api/posts';
import { getFeed } from '../api/feed';
import { ErrorCode } from '../types/api';
import type { FeedItem } from '../types/post';
import PostList from '../components/PostList';
import LoadingSpinner from '../components/LoadingSpinner';
import ErrorMessage from '../components/ErrorMessage';
import EmptyState from '../components/EmptyState';

export default function PostListPage() {
  const [order, setOrder] = useState<'time' | 'score'>('time');
  const [items, setItems] = useState<FeedItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async (sortOrder: 'time' | 'score') => {
    setIsLoading(true);
    setError(null);
    try {
      // Use feed with score for "hot" ordering
      if (sortOrder === 'score') {
        const res = await getFeed(1, 20);
        if (res.code === ErrorCode.Success) {
          setItems(res.data.items);
        } else {
          setError(res.msg);
        }
      } else {
        // Use /api/v1/post for time-ordered list
        const res = await getPostList({ page: 1, size: 20, order: 'time' });
        if (res.code === ErrorCode.Success) {
          setItems(res.data.map((p) => ({
            post_id: p.post_id,
            title: p.title || '',
            description: p.description || '',
            author_id: p.author_id,
            author_name: p.author_name,
            like_count: p.vote_num,
            favorite_count: null,
            publish_time: p.publish_time || '',
            create_time: p.create_time || '',
          })));
        } else {
          setError(res.msg);
        }
      }
    } catch (err: any) {
      setError(err.message || '加载失败');
    }
    setIsLoading(false);
  };

  useEffect(() => {
    fetchData(order);
  }, [order]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-scholar-text">发现</h1>
        <div className="flex bg-white border border-scholar-border rounded-lg overflow-hidden">
          <button
            onClick={() => setOrder('time')}
            className={`px-4 py-1.5 text-sm transition-colors ${
              order === 'time' ? 'bg-scholar-accent text-white' : 'text-scholar-muted hover:text-scholar-text'
            }`}
          >
            最新
          </button>
          <button
            onClick={() => setOrder('score')}
            className={`px-4 py-1.5 text-sm transition-colors ${
              order === 'score' ? 'bg-scholar-accent text-white' : 'text-scholar-muted hover:text-scholar-text'
            }`}
          >
            热门
          </button>
        </div>
      </div>

      {isLoading ? (
        <LoadingSpinner />
      ) : error ? (
        <ErrorMessage message={error} onRetry={() => fetchData(order)} />
      ) : items.length === 0 ? (
        <EmptyState title="暂无文章" description="还没有已发布的文章" />
      ) : (
        <PostList items={items} />
      )}
    </div>
  );
}
