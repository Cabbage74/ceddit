import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Calendar } from 'lucide-react';
import { usePostStore } from '../stores/postStore';
import { useAuthStore } from '../stores/authStore';
import { followUser, unfollowUser, getFollowingList, getFollowerList } from '../api/relation';
import { uuidv4 } from '../utils/uuid';
import { ErrorCode } from '../types/api';
import MarkdownRenderer from '../components/MarkdownRenderer';
import LikeButton from '../components/LikeButton';
import FollowButton from '../components/FollowButton';
import ChatPanel from '../components/ChatPanel';
import LoadingSpinner from '../components/LoadingSpinner';
import ErrorMessage from '../components/ErrorMessage';

export default function PostDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { currentPost, isLoading, error, fetchPostDetail, toggleLike } = usePostStore();
  const { isLoggedIn, userID, followingSet, setFollowing } = useAuthStore();

  const [liked, setLiked] = useState(false);
  const [likeCount, setLikeCount] = useState(0);
  const [likeLoading, setLikeLoading] = useState(false);
  const [followLoading, setFollowLoading] = useState(false);
  const [followingCount, setFollowingCount] = useState(0);
  const [followerCount, setFollowerCount] = useState(0);

  useEffect(() => {
    if (id) fetchPostDetail(id);
  }, [id, fetchPostDetail]);

  useEffect(() => {
    if (currentPost) setLikeCount(currentPost.vote_num);
  }, [currentPost]);

  const authorId = currentPost?.author_id;
  const isFollowing = authorId ? followingSet.has(authorId) : false;
  const isOwnPost = !!(userID && authorId && userID === authorId);

  // Fetch counts and sync follow state from server
  useEffect(() => {
    if (!authorId || !isLoggedIn || !userID) return;

    (async () => {
      try {
        const fRes = await getFollowerList(authorId, undefined, 1);
        if (fRes.code === ErrorCode.Success) setFollowerCount(fRes.data.total);
      } catch {}

      try {
        const gRes = await getFollowingList(userID, undefined, 100);
        if (gRes.code === ErrorCode.Success) {
          setFollowingCount(gRes.data.total);
          // Sync global cache with server state
          for (const item of gRes.data.list) {
            setFollowing(item.user_id, true);
          }
        }
      } catch {}
    })();
  }, [authorId, isLoggedIn, userID, setFollowing]);

  const handleLike = async () => {
    if (!id || likeLoading) return;
    setLikeLoading(true);
    const newLiked = await toggleLike(id, liked);
    setLiked(newLiked);
    setLikeCount((c) => c + (newLiked ? 1 : -1));
    setLikeLoading(false);
  };

  const handleFollowToggle = async () => {
    if (!authorId || followLoading || !isLoggedIn) return;
    setFollowLoading(true);
    try {
      const key = uuidv4();
      if (isFollowing) {
        const res = await unfollowUser(authorId, key);
        if (res.code === ErrorCode.Success) {
          setFollowing(authorId, false);
          setFollowerCount((c) => Math.max(0, c - 1));
        }
      } else {
        const res = await followUser(authorId, key);
        if (res.code === ErrorCode.Success) {
          setFollowing(authorId, true);
          setFollowerCount((c) => c + 1);
        }
      }
    } catch (err) {
      console.error('Follow toggle failed:', err);
    }
    setFollowLoading(false);
  };

  if (isLoading) return <LoadingSpinner />;
  if (error) return <ErrorMessage message={error} onRetry={() => id && fetchPostDetail(id)} />;
  if (!currentPost) return <ErrorMessage message="文章不存在" />;

  return (
    <div>
      <button onClick={() => navigate(-1)}
        className="flex items-center gap-1 text-sm text-scholar-muted hover:text-scholar-text mb-4 transition-colors">
        <ArrowLeft className="w-4 h-4" /> 返回
      </button>

      <article className="bg-white border border-scholar-border rounded-lg p-6 mb-4">
        <h1 className="text-2xl font-bold text-scholar-text mb-4 leading-snug">
          {currentPost.title || '无标题'}
        </h1>

        <div className="flex items-center justify-between mb-6 pb-4 border-b border-scholar-border">
          <div className="flex items-center gap-4">
            <div className="w-11 h-11 rounded-full bg-scholar-accent/10 flex items-center justify-center text-scholar-accent font-bold text-lg">
              {currentPost.author_name.charAt(0).toUpperCase()}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <button onClick={() => navigate(`/users/${authorId}/following`)}
                  className="font-semibold text-scholar-text hover:text-scholar-accent transition-colors">
                  {currentPost.author_name}
                </button>
                {isOwnPost && <span className="text-xs text-scholar-muted bg-scholar-bg px-2 py-0.5 rounded-full">我</span>}
              </div>
              <div className="flex items-center gap-3 text-xs text-scholar-muted mt-0.5">
                {currentPost.publish_time && (
                  <span className="flex items-center gap-1">
                    <Calendar className="w-3 h-3" />
                    {new Date(currentPost.publish_time).toLocaleDateString('zh-CN')}
                  </span>
                )}
                <button onClick={() => navigate(`/users/${authorId}/following`)}
                  className="hover:text-scholar-accent transition-colors">
                  <span className="font-medium text-scholar-text">{followingCount || '-'}</span> 关注
                </button>
                <button onClick={() => navigate(`/users/${authorId}/followers`)}
                  className="hover:text-scholar-accent transition-colors">
                  <span className="font-medium text-scholar-text">{followerCount || '-'}</span> 粉丝
                </button>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <LikeButton liked={liked} likeCount={likeCount} onToggle={handleLike} disabled={likeLoading} />
            {!isOwnPost && isLoggedIn && (
              <FollowButton isFollowing={isFollowing} onToggle={handleFollowToggle} disabled={followLoading} />
            )}
          </div>
        </div>

        <MarkdownRenderer content={currentPost.content || '*暂无内容*'} />
      </article>

      {id && <ChatPanel postId={id} />}
    </div>
  );
}
