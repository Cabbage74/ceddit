import { Link } from 'react-router-dom';
import { Heart, MessageCircle } from 'lucide-react';
import type { FeedItem } from '../types/post';

interface Props {
  item: FeedItem;
  showActions?: boolean;
}

export default function PostCard({ item, showActions = true }: Props) {
  const timeStr = item.publish_time
    ? new Date(item.publish_time).toLocaleDateString('zh-CN')
    : '';

  return (
    <article className="bg-white border border-scholar-border rounded-lg p-5 hover:border-scholar-accent/30 hover:shadow-sm transition-all">
      <Link to={`/post/${item.post_id}`} className="block">
        <h3 className="text-lg font-semibold text-scholar-text mb-2 leading-snug line-clamp-2 hover:text-scholar-accent transition-colors">
          {item.title || '无标题'}
        </h3>
        {item.description && (
          <p className="text-sm text-scholar-muted mb-3 line-clamp-2 leading-relaxed">
            {item.description}
          </p>
        )}
      </Link>

      <div className="flex items-center justify-between text-xs text-scholar-muted">
        <div className="flex items-center gap-3">
          <Link
            to={`/users/${item.author_id}/following`}
            className="hover:text-scholar-accent transition-colors font-medium"
          >
            {item.author_name}
          </Link>
          {timeStr && <span>{timeStr}</span>}
        </div>

        {showActions && (
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1">
              <Heart className={`w-3.5 h-3.5 ${item.liked ? 'fill-red-400 text-red-400' : ''}`} />
              {item.like_count ?? 0}
            </span>
            <span className="flex items-center gap-1">
              <MessageCircle className="w-3.5 h-3.5" />
              问答
            </span>
          </div>
        )}
      </div>
    </article>
  );
}
