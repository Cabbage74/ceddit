import { UserPlus, UserMinus } from 'lucide-react';

interface Props {
  isFollowing: boolean;
  onToggle: () => void;
  disabled?: boolean;
}

export default function FollowButton({ isFollowing, onToggle, disabled }: Props) {
  return (
    <button
      onClick={onToggle}
      disabled={disabled}
      className={`flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg border transition-all ${
        isFollowing
          ? 'text-scholar-muted border-scholar-border hover:border-red-200 hover:text-red-500'
          : 'text-scholar-accent border-scholar-accent/30 hover:bg-scholar-accent hover:text-white'
      } disabled:opacity-50`}
    >
      {isFollowing ? (
        <>
          <UserMinus className="w-4 h-4" />
          <span>已关注</span>
        </>
      ) : (
        <>
          <UserPlus className="w-4 h-4" />
          <span>关注</span>
        </>
      )}
    </button>
  );
}
