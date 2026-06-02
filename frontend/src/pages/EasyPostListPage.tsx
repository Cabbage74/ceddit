import { useEffect, useState } from 'react';
import { getEasyPost } from '../api/posts';
import { ErrorCode } from '../types/api';
import type { FeedItem } from '../types/post';
import PostList from '../components/PostList';
import LoadingSpinner from '../components/LoadingSpinner';
import ErrorMessage from '../components/ErrorMessage';
import EmptyState from '../components/EmptyState';

export default function EasyPostListPage() {
  const [items, setItems] = useState<FeedItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await getEasyPost(1, 20);
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
    } catch (err: any) {
      setError(err.message || '加载失败');
    }
    setIsLoading(false);
  };

  useEffect(() => { fetchData(); }, []);

  return (
    <div>
      <h1 className="text-2xl font-bold text-scholar-text mb-6">文章列表</h1>
      {isLoading ? (
        <LoadingSpinner />
      ) : error ? (
        <ErrorMessage message={error} onRetry={fetchData} />
      ) : items.length === 0 ? (
        <EmptyState title="暂无文章" />
      ) : (
        <PostList items={items} />
      )}
    </div>
  );
}
