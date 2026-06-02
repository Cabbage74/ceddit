import PostCard from './PostCard';
import type { FeedItem } from '../types/post';

interface Props {
  items: FeedItem[];
  hasMore?: boolean;
  isLoading?: boolean;
  onLoadMore?: () => void;
}

export default function PostList({ items, hasMore, isLoading, onLoadMore }: Props) {
  if (!items.length && !isLoading) {
    return null;
  }

  return (
    <div className="space-y-3">
      {items.map((item) => (
        <PostCard key={item.post_id} item={item} />
      ))}
      {hasMore && onLoadMore && (
        <div className="flex justify-center pt-4">
          <button
            onClick={onLoadMore}
            disabled={isLoading}
            className="px-6 py-2 text-sm text-scholar-accent border border-scholar-accent/30 rounded-lg hover:bg-scholar-accent/5 disabled:opacity-50 transition-colors"
          >
            {isLoading ? '加载中...' : '加载更多'}
          </button>
        </div>
      )}
    </div>
  );
}
