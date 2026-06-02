import { Heart } from 'lucide-react';

interface Props {
  liked: boolean;
  likeCount: number;
  onToggle: () => void;
  disabled?: boolean;
}

export default function LikeButton({ liked, likeCount, onToggle, disabled }: Props) {
  return (
    <button
      onClick={onToggle}
      disabled={disabled}
      className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg border transition-all ${
        liked
          ? 'text-red-500 border-red-200 bg-red-50 hover:bg-red-100'
          : 'text-scholar-muted border-scholar-border hover:border-red-200 hover:text-red-400'
      } disabled:opacity-50`}
    >
      <Heart className={`w-4 h-4 ${liked ? 'fill-current' : ''}`} />
      <span>{likeCount}</span>
    </button>
  );
}
