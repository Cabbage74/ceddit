export interface Post {
  post_id: string;
  title: string;
  description: string;
  content_object_key: string | null;
  content_etag: string | null;
  content_size: number | null;
  content_sha256: string | null;
  author_id: string;
  status: string;
  create_time: string | null;
  publish_time: string | null;
  update_time: string | null;
}

export interface PostDetail extends Post {
  author_name: string;
  vote_num: number;
  content: string;
}

export interface FeedItem {
  post_id: string;
  title: string;
  description: string;
  author_id: string;
  author_name: string;
  like_count: number | null;
  favorite_count: number | null;
  publish_time: string;
  create_time: string;
  liked?: boolean;
  faved?: boolean;
}

export interface FeedPageResponse {
  items: FeedItem[];
  page: number;
  size: number;
  has_more: boolean;
}

export interface RespCreateDraft {
  post_id: string;
}

export interface RespPresign {
  object_key: string;
  put_url: string;
  expires_in: number;
}

export interface VoteResult {
  changed: boolean;
  liked: boolean;
}

// Create post flow params
export interface ParamPresign {
  scene: 'post_content' | 'post_image';
  post_id: string;
  content_type: string;
  ext: string;
}

export interface ParamContentConfirm {
  object_key: string;
  etag: string;
  size: number;
  sha256: string;
}

export interface ParamPatchPost {
  title: string;
}

export interface ParamVote {
  post_id: string;
  direction: number; // 1 = like, 0 = neutral, -1 = dislike
}

export interface ParamPostList {
  page?: number;
  size?: number;
  order?: 'time' | 'score';
}
