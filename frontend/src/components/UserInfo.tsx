import { useNavigate } from 'react-router-dom';
import FollowButton from './FollowButton';

interface Props {
  userId: string;
  username: string;
  isFollowing: boolean;
  onToggleFollow: () => void;
  followingCount?: number;
  followerCount?: number;
}

export default function UserInfo({
  userId,
  username,
  isFollowing,
  onToggleFollow,
  followingCount,
  followerCount,
}: Props) {
  const navigate = useNavigate();

  return (
    <div className="flex items-center gap-4">
      {/* Avatar placeholder */}
      <div className="w-12 h-12 rounded-full bg-scholar-accent/10 flex items-center justify-center text-scholar-accent font-bold text-lg">
        {username.charAt(0).toUpperCase()}
      </div>

      <div className="flex-1">
        <h3 className="font-semibold text-scholar-text">{username}</h3>
        <div className="flex items-center gap-3 text-xs text-scholar-muted mt-0.5">
          <button
            onClick={() => navigate(`/users/${userId}/following`)}
            className="hover:text-scholar-accent transition-colors"
          >
            <span className="font-medium text-scholar-text">{followingCount ?? '-'}</span> 关注
          </button>
          <button
            onClick={() => navigate(`/users/${userId}/followers`)}
            className="hover:text-scholar-accent transition-colors"
          >
            <span className="font-medium text-scholar-text">{followerCount ?? '-'}</span> 粉丝
          </button>
        </div>
      </div>

      <FollowButton isFollowing={isFollowing} onToggle={onToggleFollow} />
    </div>
  );
}
